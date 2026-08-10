package vm

import (
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestDateTimeUTC(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Test 1: Basic call without array
	got, err := v.natives["DATETIMEUTC"].Fn([]advplrt.Value{})
	if err != nil {
		t.Fatalf("DateTimeUTC() retornou erro: %v", err)
	}
	s, ok := got.(*advplrt.StringValue)
	if !ok {
		t.Errorf("DateTimeUTC() retornou tipo incorreto: %v", got.Type())
	}
	// Should match pattern "YYYY-MM-DDTHH:MM:SSZ"
	if len(s.Val) != 20 || s.Val[10] != 'T' || s.Val[19] != 'Z' {
		t.Errorf("DateTimeUTC() = %q, esperado formato ISO 8601 UTC", s.Val)
	}

	// Test 2: Call with array by reference
	aData := &advplrt.ArrayValue{Elements: []advplrt.Value{}}
	got, err = v.natives["DATETIMEUTC"].Fn([]advplrt.Value{aData})
	if err != nil {
		t.Fatalf("DateTimeUTC(aData) retornou erro: %v", err)
	}
	s, ok = got.(*advplrt.StringValue)
	if !ok {
		t.Errorf("DateTimeUTC(aData) retornou tipo incorreto: %v", got.Type())
	}
	// Check that array was filled
	if len(aData.Elements) < 2 {
		t.Errorf("DateTimeUTC(aData) não preencheu array: %v", aData.Elements)
	}
	// Array[1] should be date in YYYYMMDD format
	date, ok := aData.Elements[0].(*advplrt.StringValue)
	if !ok || len(date.Val) != 8 {
		t.Errorf("DateTimeUTC(aData)[1] = %q, esperado formato YYYYMMDD", advplrt.ToString(aData.Elements[0]))
	}
	// Array[2] should be time in HH:MM:SS format
	time_val, ok := aData.Elements[1].(*advplrt.StringValue)
	if !ok || len(time_val.Val) != 8 {
		t.Errorf("DateTimeUTC(aData)[2] = %q, esperado formato HH:MM:SS", advplrt.ToString(aData.Elements[1]))
	}
}

func TestGetTimeStamp(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Use current date
	now := time.Now()
	dDate := advplrt.NewDate(now)

	// Test 1: GetTimeStamp with date only
	got, err := v.natives["GETTIMESTAMP"].Fn([]advplrt.Value{dDate})
	if err != nil {
		t.Fatalf("GetTimeStamp(dDate) retornou erro: %v", err)
	}
	s, ok := got.(*advplrt.StringValue)
	if !ok {
		t.Errorf("GetTimeStamp() retornou tipo incorreto: %v", got.Type())
	}
	// Format should be "YYYYMMDD HH:MM:SS.fff" = 21 chars
	if len(s.Val) != 21 || s.Val[8] != ' ' || s.Val[11] != ':' || s.Val[14] != ':' || s.Val[17] != '.' {
		t.Errorf("GetTimeStamp() = %q, esperado formato YYYYMMDD HH:MM:SS.fff", s.Val)
	}

	// Test 2: GetTimeStamp with array by reference
	aData := &advplrt.ArrayValue{Elements: []advplrt.Value{}}
	got, err = v.natives["GETTIMESTAMP"].Fn([]advplrt.Value{dDate, aData})
	if err != nil {
		t.Fatalf("GetTimeStamp(dDate, aData) retornou erro: %v", err)
	}
	if len(aData.Elements) < 2 {
		t.Errorf("GetTimeStamp() não preencheu array: %v", aData.Elements)
	}
	date, ok := aData.Elements[0].(*advplrt.StringValue)
	if !ok || len(date.Val) != 8 {
		t.Errorf("GetTimeStamp()[1] = %q, esperado YYYYMMDD", advplrt.ToString(aData.Elements[0]))
	}
	time_val, ok := aData.Elements[1].(*advplrt.StringValue)
	if !ok || len(time_val.Val) != 12 {
		t.Errorf("GetTimeStamp()[2] = %q, esperado HH:MM:SS.fff", advplrt.ToString(aData.Elements[1]))
	}
}

func TestLocalToUTC(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	cases := []struct {
		name    string
		date    string
		time    string
		dst     float64
		wantLen int // length of returned array
	}{
		{
			"simple conversion",
			"20080918",
			"23:24:25",
			0,
			2,
		},
	}

	for _, c := range cases {
		got, err := v.natives["LOCALTOUTC"].Fn([]advplrt.Value{
			advplrt.NewString(c.date),
			advplrt.NewString(c.time),
			advplrt.NewNumber(c.dst),
		})
		if err != nil {
			t.Fatalf("LocalToUTC(%s) retornou erro: %v", c.name, err)
		}
		arr, ok := got.(*advplrt.ArrayValue)
		if !ok || len(arr.Elements) != c.wantLen {
			t.Errorf("LocalToUTC(%s) retornou tipo incorreto ou tamanho: %v", c.name, got)
		}
		// Check that array elements are strings in correct format
		date, ok := arr.Elements[0].(*advplrt.StringValue)
		if !ok || len(date.Val) != 8 {
			t.Errorf("LocalToUTC()[1] = %q, esperado YYYYMMDD", advplrt.ToString(arr.Elements[0]))
		}
		time_val, ok := arr.Elements[1].(*advplrt.StringValue)
		if !ok || len(time_val.Val) != 8 {
			t.Errorf("LocalToUTC()[2] = %q, esperado HH:MM:SS", advplrt.ToString(arr.Elements[1]))
		}
	}
}

func TestTimecounter(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Test 1: Single call returns a number
	got1, err := v.natives["TIMECOUNTER"].Fn([]advplrt.Value{})
	if err != nil {
		t.Fatalf("timecounter() retornou erro: %v", err)
	}
	n1, ok := got1.(*advplrt.NumberValue)
	if !ok {
		t.Errorf("timecounter() retornou tipo incorreto: %v", got1.Type())
	}

	// Test 2: Multiple calls should be monotonically increasing
	got2, err := v.natives["TIMECOUNTER"].Fn([]advplrt.Value{})
	if err != nil {
		t.Fatalf("timecounter() segunda chamada retornou erro: %v", err)
	}
	n2, ok := got2.(*advplrt.NumberValue)
	if !ok {
		t.Errorf("timecounter() segunda chamada retornou tipo incorreto: %v", got2.Type())
	}

	// n2 should be >= n1 (can be equal due to timing)
	if n2.Val < n1.Val {
		t.Errorf("timecounter() não é monotônico: %v >= %v", n2.Val, n1.Val)
	}
}

func TestTimeFull(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	got, err := v.natives["TIMEFULL"].Fn([]advplrt.Value{})
	if err != nil {
		t.Fatalf("TimeFull() retornou erro: %v", err)
	}
	s, ok := got.(*advplrt.StringValue)
	if !ok {
		t.Errorf("TimeFull() retornou tipo incorreto: %v", got.Type())
	}
	// Format should be "HH:MM:SS.fff"
	if len(s.Val) != 12 || s.Val[2] != ':' || s.Val[5] != ':' || s.Val[8] != '.' {
		t.Errorf("TimeFull() = %q, esperado formato HH:MM:SS.fff", s.Val)
	}
}

func TestUnixMS2DT(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	cases := []struct {
		name    string
		unixTS  float64
		wantLen int
	}{
		{
			"TDN example",
			1293885296.789,
			21, // "20110101 12:34:56.789" = 21 chars (YYYYMMDD HH:MM:SS.fff)
		},
		{
			"epoch",
			0,
			21,
		},
	}

	for _, c := range cases {
		got, err := v.natives["UNIXMS2DT"].Fn([]advplrt.Value{
			advplrt.NewNumber(c.unixTS),
		})
		if err != nil {
			t.Fatalf("UnixMS2DT(%s) retornou erro: %v", c.name, err)
		}
		s, ok := got.(*advplrt.StringValue)
		if !ok {
			t.Errorf("UnixMS2DT(%s) retornou tipo incorreto: %v", c.name, got.Type())
			continue
		}
		if len(s.Val) != c.wantLen {
			t.Errorf("UnixMS2DT(%s) = %q, esperado comprimento %d", c.name, s.Val, c.wantLen)
		}
		// Verify format: "YYYYMMDD HH:MM:SS.fff"
		if len(s.Val) >= 21 && (s.Val[8] != ' ' || s.Val[11] != ':' || s.Val[14] != ':' || s.Val[17] != '.') {
			t.Errorf("UnixMS2DT(%s) = %q, formato incorreto", c.name, s.Val)
		}
	}
}

func TestUTCToLocal(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	cases := []struct {
		name    string
		date    string
		time    string
		dst     float64
		wantLen int // length of returned array
	}{
		{
			"simple conversion",
			"20080919",
			"02:24:25",
			0,
			2,
		},
	}

	for _, c := range cases {
		got, err := v.natives["UTCTOLOCAL"].Fn([]advplrt.Value{
			advplrt.NewString(c.date),
			advplrt.NewString(c.time),
			advplrt.NewNumber(c.dst),
		})
		if err != nil {
			t.Fatalf("UTCToLocal(%s) retornou erro: %v", c.name, err)
		}
		arr, ok := got.(*advplrt.ArrayValue)
		if !ok || len(arr.Elements) != c.wantLen {
			t.Errorf("UTCToLocal(%s) retornou tipo incorreto ou tamanho: %v", c.name, got)
		}
		// Check that array elements are strings in correct format
		date, ok := arr.Elements[0].(*advplrt.StringValue)
		if !ok || len(date.Val) != 8 {
			t.Errorf("UTCToLocal()[1] = %q, esperado YYYYMMDD", advplrt.ToString(arr.Elements[0]))
		}
		time_val, ok := arr.Elements[1].(*advplrt.StringValue)
		if !ok || len(time_val.Val) != 8 {
			t.Errorf("UTCToLocal()[2] = %q, esperado HH:MM:SS", advplrt.ToString(arr.Elements[1]))
		}
	}
}

// TestLocalToUTCDST reproduz os exemplos documentados do TDN, que devem valer
// em qualquer fuso local (o TDN assume offset solar fixo + DST manual via nDST):
//
//	LocalToUTC("20130110","13:00:00",1) -> 20130110 15:00:00 UTC
//	LocalToUTC("20130110","12:00:00",0) -> 20130110 15:00:00 UTC
func TestLocalToUTCDST(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Com nDST=1 em 13:00:00 o resultado deve ser o mesmo que nDST=0 em 12:00:00
	aDst, err := v.natives["LOCALTOUTC"].Fn([]advplrt.Value{
		advplrt.NewString("20130110"),
		advplrt.NewString("13:00:00"),
		advplrt.NewNumber(1),
	})
	if err != nil {
		t.Fatalf("LocalToUTC(13:00, nDST=1) retornou erro: %v", err)
	}
	aStd, err := v.natives["LOCALTOUTC"].Fn([]advplrt.Value{
		advplrt.NewString("20130110"),
		advplrt.NewString("12:00:00"),
		advplrt.NewNumber(0),
	})
	if err != nil {
		t.Fatalf("LocalToUTC(12:00, nDST=0) retornou erro: %v", err)
	}

	dstArr := aDst.(*advplrt.ArrayValue)
	stdArr := aStd.(*advplrt.ArrayValue)
	if advplrt.ToString(dstArr.Elements[0]) != advplrt.ToString(stdArr.Elements[0]) ||
		advplrt.ToString(dstArr.Elements[1]) != advplrt.ToString(stdArr.Elements[1]) {
		t.Errorf("LocalToUTC nDST=1(13:00) != nDST=0(12:00): %v vs %v",
			advplrt.ToString(aDst), advplrt.ToString(aStd))
	}

	// Sem nDST, 12:00 local na zona atual (America/Sao_Paulo = -03) deve dar 15:00 UTC
	// — mas o offset do fuso local varia por máquina, então validamos a invariante
	// genérica: resultado == hora local + offset solar.
	off := localStdOffset()
	want := "20130110 12:00:00"
	// Reconstroi o esperado a partir do offset solar do fuso local
	base, _ := time.ParseInLocation("20060102 15:04:05", want, time.UTC)
	utcWant := base.Add(-time.Duration(off) * time.Second).UTC().Format("20060102 15:04:05")

	stdArr2 := aStd.(*advplrt.ArrayValue)
	got := advplrt.ToString(stdArr2.Elements[0]) + " " + advplrt.ToString(stdArr2.Elements[1])
	if got != utcWant {
		t.Errorf("LocalToUTC(20130110,12:00,0) = %q, esperado %q", got, utcWant)
	}
}

// TestUTCToLocalDST reproduz os exemplos documentados do TDN:
//
//	UTCToLocal("20130110","15:00:00",0) -> 12:00:00 local (solar)
//	UTCToLocal("20130110","15:00:00",1) -> 13:00:00 local (DST)
func TestUTCToLocalDST(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// 15:00 UTC com nDST=0 -> hora solar local = 15:00 - offsetSolar
	off := localStdOffset()
	utcTime, _ := time.ParseInLocation("20060102 15:04:05", "20130110 15:00:00", time.UTC)
	solarWant := utcTime.Add(time.Duration(off) * time.Second).Format("20060102 15:04:05")

	got, err := v.natives["UTCTOLOCAL"].Fn([]advplrt.Value{
		advplrt.NewString("20130110"),
		advplrt.NewString("15:00:00"),
		advplrt.NewNumber(0),
	})
	if err != nil {
		t.Fatalf("UTCToLocal(15:00, nDST=0) retornou erro: %v", err)
	}
	solarArr0 := got.(*advplrt.ArrayValue)
	solarGot := advplrt.ToString(solarArr0.Elements[0]) + " " + advplrt.ToString(solarArr0.Elements[1])
	if solarGot != solarWant {
		t.Errorf("UTCToLocal(20130110,15:00,0) = %q, esperado %q", solarGot, solarWant)
	}

	// nDST=1 -> adiciona 1 hora ao resultado solar
	gotDst, err := v.natives["UTCTOLOCAL"].Fn([]advplrt.Value{
		advplrt.NewString("20130110"),
		advplrt.NewString("15:00:00"),
		advplrt.NewNumber(1),
	})
	if err != nil {
		t.Fatalf("UTCToLocal(15:00, nDST=1) retornou erro: %v", err)
	}
	// O resultado DST é solar+1h: validamos a relação exata.
	dstArr := gotDst.(*advplrt.ArrayValue)
	solarArr := got.(*advplrt.ArrayValue)
	dstDT, err := time.ParseInLocation("20060102 15:04:05",
		advplrt.ToString(dstArr.Elements[0])+" "+advplrt.ToString(dstArr.Elements[1]), time.UTC)
	if err != nil {
		t.Fatalf("parse do resultado DST falhou: %v", err)
	}
	solarDT, err := time.ParseInLocation("20060102 15:04:05",
		advplrt.ToString(solarArr.Elements[0])+" "+advplrt.ToString(solarArr.Elements[1]), time.UTC)
	if err != nil {
		t.Fatalf("parse do resultado solar falhou: %v", err)
	}
	if solarDT.Add(time.Hour) != dstDT {
		t.Errorf("UTCToLocal nDST=1 = %v, esperado solar+1h = %v",
			dstDT.Format("20060102 15:04:05"), solarDT.Add(time.Hour).Format("20060102 15:04:05"))
	}
}

// TestTimecounterMonotonic garante que timecounter não retorna um horário
// absoluto (TDN) e que cresce monotonicamente entre chamadas.
func TestTimecounterMonotonic(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	got1, err := v.natives["TIMECOUNTER"].Fn(nil)
	if err != nil {
		t.Fatalf("timecounter() retornou erro: %v", err)
	}
	n1 := got1.(*advplrt.NumberValue).Val

	// Valor inicial deve ser pequeno (relativo à referência de registro), não um
	// epoch gigante — evidencia que não é horário absoluto.
	if n1 < 0 || n1 > 1e6 {
		t.Errorf("timecounter() = %v, esperado valor relativo pequeno (não absoluto)", n1)
	}

	time.Sleep(5 * time.Millisecond)
	got2, err := v.natives["TIMECOUNTER"].Fn(nil)
	if err != nil {
		t.Fatalf("timecounter() segunda chamada retornou erro: %v", err)
	}
	n2 := got2.(*advplrt.NumberValue).Val
	if n2 <= n1 {
		t.Errorf("timecounter() não é monotônico: %v depois de %v", n2, n1)
	}
}
