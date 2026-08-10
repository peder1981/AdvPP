package vm

import (
	"fmt"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// localStdOffset retorna o offset solar (standard time) do fuso local, em
// segundos, ignorando regras históricas de horário de verão. É a base que o
// TDN espera para LocalToUTC/UTCToLocal: a zona do SO tratada como offset
// fixo e o DST aplicado manualmente via parâmetro nDST.
func localStdOffset() int {
	_, oJan := time.Date(2000, 1, 1, 12, 0, 0, 0, time.Local).Zone()
	_, oJul := time.Date(2000, 7, 1, 12, 0, 0, 0, time.Local).Zone()
	if oJul < oJan {
		return oJul
	}
	return oJan
}

// setArrIndex grava o valor na posição de array (0-based) informada,
// crescendo o array com Nil se necessário — semântica do TDN de "posição 1
// será data e posição 2 a hora" (o array é passado por referência).
func setArrIndex(a *advplrt.ArrayValue, idx int, v advplrt.Value) {
	for len(a.Elements) <= idx {
		a.Elements = append(a.Elements, advplrt.Nil)
	}
	a.Elements[idx] = v
}

// registerManipulacaodeDataHoraNatives registra funções de manipulação de
// data e hora: DateTimeUTC, GetTimeStamp, LocalToUTC, timecounter, TimeFull,
// UnixMS2DT, UTCToLocal.
func (v *VM) registerManipulacaodeDataHoraNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// DateTimeUTC([aDate]) -> cDateTimeUTC — retorna string UTC no formato ISO 8601, opcionalmente preenchendo array
	natives["DATETIMEUTC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		now := time.Now().UTC()
		// Formato ISO 8601: "2019-05-16T15:10:14Z"
		cDateTimeUTC := now.Format("2006-01-02T15:04:05Z")

		// Se array foi passado por referência, preenchê-lo
		if len(args) > 0 {
			if aDate, ok := getArg(args, 0).(*advplrt.ArrayValue); ok && aDate != nil {
				// Posição 1: data em YYYYMMDD
				setArrIndex(aDate, 0, advplrt.NewString(now.Format("20060102")))
				// Posição 2: hora em HH:MM:SS
				setArrIndex(aDate, 1, advplrt.NewString(now.Format("15:04:05")))
			}
		}

		return advplrt.NewString(cDateTimeUTC), nil
	}

	// GetTimeStamp(dDate, [aDate]) -> cTimeStamp — retorna string com timestamp no formato YYYYMMDD HH:MM:SS.fff
	natives["GETTIMESTAMP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		// Obter data do primeiro argumento
		dDate := getArg(args, 0)
		var t time.Time

		if d, ok := dDate.(*advplrt.DateValue); ok {
			t = d.Val
		} else {
			// Se não for DateValue, usar agora
			t = time.Now()
		}

		// Formato: "YYYYMMDD HH:MM:SS.fff"
		cTimeStamp := t.Format("20060102 15:04:05.000")

		// Se array foi passado por referência, preenchê-lo
		if len(args) > 1 {
			if aDate, ok := getArg(args, 1).(*advplrt.ArrayValue); ok && aDate != nil {
				// Posição 1: data em YYYYMMDD
				setArrIndex(aDate, 0, advplrt.NewString(t.Format("20060102")))
				// Posição 2: hora em HH:MM:SS.fff
				setArrIndex(aDate, 1, advplrt.NewString(t.Format("15:04:05.000")))
			}
		}

		return advplrt.NewString(cTimeStamp), nil
	}

	// LocalToUTC(cDate, cTime, [nDST]) -> aRet — converte data/hora local para UTC
	natives["LOCALTOUTC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cDate := advplrt.ToString(getArg(args, 0))
		cTime := advplrt.ToString(getArg(args, 1))
		nDST := int(advplrt.ToFloat(getArg(args, 2)))

		// Interpreta a data/hora no offset solar (standard time) fixo do fuso
		// local — NÃO em time.Local com regras históricas de DST, pois o TDN
		// define que o DST é aplicado exclusivamente via parâmetro nDST.
		t, err := time.ParseInLocation("20060102 15:04:05", cDate+" "+cTime, time.FixedZone("Local", localStdOffset()))
		if err != nil {
			// Return array with empty/nil on parse error
			return advplrt.NewArray([]advplrt.Value{
				advplrt.NewString(""),
				advplrt.NewString(""),
			}), nil
		}

		// Se nDST == 1, assumir que a hora está em DST (daylight saving time)
		// Neste caso, subtrair 1 hora antes de converter para UTC
		if nDST == 1 {
			t = t.Add(-1 * time.Hour)
		}

		// Converter para UTC
		utc := t.UTC()

		// Retornar array com data e hora UTC
		return advplrt.NewArray([]advplrt.Value{
			advplrt.NewString(utc.Format("20060102")),
			advplrt.NewString(utc.Format("15:04:05")),
		}), nil
	}

	// timecounter() -> nRet — retorna contador de tempo em milissegundos (monotônico)
	// TDN: "O valor retornado não representa um horário absoluto, devendo ser
	// utilizado apenas para comparação entre duas chamadas da função." Usa o
	// relógio monotônico do Go a partir de uma referência capturada no registro.
	timecounterBase := time.Now()
	natives["TIMECOUNTER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		elapsed := time.Since(timecounterBase)
		// Converter para milissegundos
		milliseconds := float64(elapsed) / float64(time.Millisecond)
		return advplrt.NewNumber(milliseconds), nil
	}

	// TimeFull() -> nRet — retorna hora atual no formato HH:MM:SS.fff (string, não numérico apesar do TDN dizer "numérico")
	natives["TIMEFULL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		now := time.Now()
		// Formato: "HH:MM:SS.fff"
		timeStr := now.Format("15:04:05.000")
		return advplrt.NewString(timeStr), nil
	}

	// UnixMS2DT(nUnixTS) -> cRet — converte Unix timestamp (segundos desde 1970-01-01) para string YYYYMMDD HH:MM:SS.fff
	natives["UNIXMS2DT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		nUnixTS := advplrt.ToFloat(getArg(args, 0))

		// Separar segundos e milissegundos
		seconds := int64(nUnixTS)
		millis := int((nUnixTS - float64(seconds)) * 1000)

		// Criar time a partir do Unix timestamp
		t := time.Unix(seconds, int64(millis)*1e6).UTC()

		// Formato: "YYYYMMDD HH:MM:SS.fff"
		cRet := fmt.Sprintf("%s.%03d", t.Format("20060102 15:04:05"), millis)
		return advplrt.NewString(cRet), nil
	}

	// UTCToLocal(cDate, cTime, [nDST]) -> aRet — converte data/hora UTC para local
	natives["UTCTOLOCAL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cDate := advplrt.ToString(getArg(args, 0))
		cTime := advplrt.ToString(getArg(args, 1))
		nDST := int(advplrt.ToFloat(getArg(args, 2)))

		// Parser data UTC: YYYYMMDD
		t, err := time.ParseInLocation("20060102 15:04:05", cDate+" "+cTime, time.UTC)
		if err != nil {
			// Return array with empty/nil on parse error
			return advplrt.NewArray([]advplrt.Value{
				advplrt.NewString(""),
				advplrt.NewString(""),
			}), nil
		}

		// Converter para o offset solar (standard time) fixo do fuso local —
		// não para time.Local com DST histórico, pois o TDN aplica o DST
		// exclusivamente via parâmetro nDST.
		local := t.In(time.FixedZone("Local", localStdOffset()))

		// Se nDST == 1, adicionar 1 hora para representar DST
		if nDST == 1 {
			local = local.Add(1 * time.Hour)
		}

		// Retornar array com data e hora local
		return advplrt.NewArray([]advplrt.Value{
			advplrt.NewString(local.Format("20060102")),
			advplrt.NewString(local.Format("15:04:05")),
		}), nil
	}
}
