package vm

import (
	"encoding/binary"
	"image"
	"image/jpeg"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
	"github.com/jsummers/gobmp"
)

// oledateEpoch é a base do serial de data do Protheus: dias desde 1899-12-30
// (formato OLE Automation), como usado por Dbl2Dt/Dt2Dbl (Task 27).
var oledateEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// registerConversaotiposNatives registra as funções de Conversão entre tipos.
// __HEXTODEC é stub confirmado (docs/tdn-gap-stubs.md) — não registrado.
// Pré-existentes (não duplicadas): CToD, cValToChar, DToC, DToS, Str,
// StrZero, Val.
func (v *VM) registerConversaotiposNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// Bin2D( < cString > ) -> nRet — LE float64 dos 8 primeiros bytes.
	natives["BIN2D"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := binBytes(getArg(args, 0), 8)
		return advplrt.NewNumber(math.Float64frombits(binary.LittleEndian.Uint64(b))), nil
	}

	// Bin2F( < cString > ) -> nRet — LE float32 dos 4 primeiros bytes.
	natives["BIN2F"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := binBytes(getArg(args, 0), 4)
		return advplrt.NewNumber(float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))), nil
	}

	// Bin2I( < cString > ) -> nRet — LE int16 sinalizado dos 2 primeiros bytes.
	natives["BIN2I"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := binBytes(getArg(args, 0), 2)
		return advplrt.NewNumber(float64(int16(binary.LittleEndian.Uint16(b)))), nil
	}

	// Bin2L( < cString > ) -> nRet — LE int32 sinalizado dos 4 primeiros bytes.
	natives["BIN2L"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := binBytes(getArg(args, 0), 4)
		return advplrt.NewNumber(float64(int32(binary.LittleEndian.Uint32(b)))), nil
	}

	// Bin2Str( < cString > ) -> cRet — string com o valor binário de cada
	// caractere: espaço para 0, "x" para 1 (MSB primeiro).
	natives["BIN2STR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := advplrt.ToString(getArg(args, 0))
		var sb strings.Builder
		for i := 0; i < len(s); i++ {
			for bit := 7; bit >= 0; bit-- {
				if s[i]&(1<<uint(bit)) != 0 {
					sb.WriteByte('x')
				} else {
					sb.WriteByte(' ')
				}
			}
		}
		return advplrt.NewString(sb.String()), nil
	}

	// Bin2W( < cString > ) -> nRet — LE uint16 dos 2 primeiros bytes.
	natives["BIN2W"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := binBytes(getArg(args, 0), 2)
		return advplrt.NewNumber(float64(binary.LittleEndian.Uint16(b))), nil
	}

	// BmpToJpg( < cFileOld>, < cFileNew>, [lTimeOut] ) -> nRet — converte BMP
	// em JPG via gobmp (decode) + image/jpeg (encode). Retorna 0 em sucesso,
	// -1 em erro. lTimeOut=.T. converte os caminhos para minúsculas.
	natives["BMPTOJPG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		oldPath := advplrt.ToString(getArg(args, 0))
		newPath := advplrt.ToString(getArg(args, 1))
		if advplrt.ToBool(getArg(args, 2)) {
			oldPath = strings.ToLower(oldPath)
			newPath = strings.ToLower(newPath)
		}
		f, err := os.Open(oldPath)
		if err != nil {
			return advplrt.NewNumber(-1), nil
		}
		defer f.Close()
		src, err := gobmp.Decode(f)
		if err != nil {
			return advplrt.NewNumber(-1), nil
		}
		// Normaliza qualquer tipo de imagem para RGBA antes de salvar como JPG.
		var img image.Image = src
		if _, ok := src.(*image.RGBA); !ok {
			rgba := image.NewRGBA(src.Bounds())
			for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
				for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
					rgba.Set(x, y, src.At(x, y))
				}
			}
			img = rgba
		}
		out, err := os.Create(newPath)
		if err != nil {
			return advplrt.NewNumber(-1), nil
		}
		defer out.Close()
		if err := jpeg.Encode(out, img, &jpeg.Options{Quality: 85}); err != nil {
			return advplrt.NewNumber(-1), nil
		}
		return advplrt.NewNumber(0), nil
	}

	// ColorToRGB( < nColor > ) -> aRet — vetor {R, G, B, Alpha} (0-255).
	// Formato TOTVS: 0x00BBGGRR (COLORREF), confirmado com colors.ch real
	// (CLR_HBLUE=16711680=0xFF0000 -> RGB(0,0,255)).
	natives["COLORTORGB"] = func(args []advplrt.Value) (advplrt.Value, error) {
		n := uint32(advplrt.ToFloat(getArg(args, 0)))
		return advplrt.NewArray([]advplrt.Value{
			advplrt.NewNumber(float64(n & 0xFF)),         // R
			advplrt.NewNumber(float64((n >> 8) & 0xFF)),  // G
			advplrt.NewNumber(float64((n >> 16) & 0xFF)), // B
			advplrt.NewNumber(float64((n >> 24) & 0xFF)), // Alpha
		}), nil
	}

	// D2Bin( < nDouble > ) -> cRet — string de 8 bytes, LE float64.
	natives["D2BIN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, math.Float64bits(advplrt.ToFloat(getArg(args, 0))))
		return advplrt.NewString(string(b)), nil
	}

	// Dbl2Dt( < nDt > ) -> cRet — "YYYYMMDD hh:mm:ss.fff" a partir do serial
	// OLE (inteiro = dias desde 1899-12-30, fração = ms/86400000).
	natives["DBL2DT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		n := advplrt.ToFloat(getArg(args, 0))
		days := int64(math.Floor(n))
		frac := n - float64(days)
		if frac < 0 {
			days--
			frac++
		}
		ms := int64(math.Round(frac * 86400000))
		// Overflow de ms no dia -> rola para o dia seguinte (hora 24:00:00).
		if ms >= 86400000 {
			days++
			ms -= 86400000
		}
		t := oledateEpoch.AddDate(0, 0, int(days)).Add(time.Duration(ms) * time.Millisecond)
		return advplrt.NewString(t.Format("20060102 15:04:05.000")), nil
	}

	// Dt2Dbl( < cExp > ) -> dblRet — serial OLE (dias + ms/86400000) a partir
	// de "YYYYMMDD hh:mm:ss.fff".
	natives["DT2DBL"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := strings.TrimSpace(advplrt.ToString(getArg(args, 0)))
		var datePart, timePart string
		if i := strings.IndexByte(s, ' '); i >= 0 {
			datePart, timePart = s[:i], s[i+1:]
		} else {
			datePart = s
		}
		t, err := time.ParseInLocation("20060102", datePart, time.UTC)
		if err != nil {
			return advplrt.NewNumber(0), nil
		}
		days := float64(t.Sub(oledateEpoch) / (24 * time.Hour))
		ms := float64(0)
		if timePart != "" {
			// "hh:mm:ss.fff"
			tp := timePart
			var msPart string
			if i := strings.IndexByte(tp, '.'); i >= 0 {
				msPart = tp[i+1:]
				tp = tp[:i]
			}
			hm, err := time.Parse("15:04:05", tp)
			if err != nil {
				// tenta só hh:mm
				hm, err = time.Parse("15:04", tp)
				if err != nil {
					return advplrt.NewNumber(0), nil
				}
			}
			ms = float64(hm.Hour()*3600000 + hm.Minute()*60000 + hm.Second()*1000)
			if msPart != "" {
				// usa os dígitos após o ponto como fração de milissegundo
				msStr := msPart
				for len(msStr) < 3 {
					msStr += "0"
				}
				if f, err := strconv.Atoi(msStr[:3]); err == nil {
					ms += float64(f)
				}
			}
		}
		return advplrt.NewNumber(days + ms/86400000), nil
	}

	// F2Bin( < nFloat > ) -> cRet — string de 4 bytes, LE float32.
	natives["F2BIN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, math.Float32bits(float32(advplrt.ToFloat(getArg(args, 0)))))
		return advplrt.NewString(string(b)), nil
	}

	// GetDtoDate( < cData > ) -> dRet — converte string em data; aceita
	// "mm/dd/yy" (como CToD) e também sem separadores "mmddyy" (formato US,
	// per exemplo TDN: "02/16/05" -> 16/fevereiro/2005).
	natives["GETDTODATE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		s := strings.TrimSpace(advplrt.ToString(getArg(args, 0)))
		if s == "" {
			return advplrt.Nil, nil
		}
		var t time.Time
		var err error
		if strings.ContainsAny(s, "/-.") {
			t, err = time.ParseInLocation("01/02/06", s, time.Local)
			if err == nil {
				return advplrt.NewDate(t), nil
			}
			t, err = time.ParseInLocation("01/02/2006", s, time.Local)
		} else {
			// sem separadores: "mmddyy" (mesmo formato US)
			t, err = time.ParseInLocation("010206", s, time.Local)
		}
		if err != nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewDate(t), nil
	}

	// I2Bin( < nInt > ) -> cRet — string de 2 bytes, LE int16 (trunca
	// decimais).
	natives["I2BIN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, uint16(int16(advplrt.ToFloat(getArg(args, 0)))))
		return advplrt.NewString(string(b)), nil
	}

	// L2Bin( < nInt > ) -> cRet — string de 4 bytes, LE int32 (trunca
	// decimais).
	natives["L2BIN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, uint32(int32(advplrt.ToFloat(getArg(args, 0)))))
		return advplrt.NewString(string(b)), nil
	}

	// W2Bin( < nInt > ) -> cRet — string de 2 bytes, LE uint16 (trunca
	// decimais).
	natives["W2BIN"] = func(args []advplrt.Value) (advplrt.Value, error) {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, uint16(advplrt.ToFloat(getArg(args, 0))))
		return advplrt.NewString(string(b)), nil
	}
}

// binBytes devolve os primeiros n bytes da string, preenchendo com 0x00 se a
// string for mais curta (comportamento Clipper/Harbour de Bin2*).
func binBytes(v advplrt.Value, n int) []byte {
	s := advplrt.ToString(v)
	b := make([]byte, n)
	for i := 0; i < n && i < len(s); i++ {
		b[i] = s[i]
	}
	return b
}
