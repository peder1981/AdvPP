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
