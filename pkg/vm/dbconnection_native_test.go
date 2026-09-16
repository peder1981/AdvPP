package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestDbConnectionNewSetsFields(t *testing.T) {
	resetDbaccessState()
	v := NewVM(&compiler.Bytecode{}, false)
	obj := newDbConnectionObject()
	args := []advplrt.Value{
		advplrt.NewString("POSTGRES"),
		advplrt.NewString("10.0.0.5"),
		advplrt.NewNumber(5432),
		advplrt.NewString("meubanco"),
		advplrt.NewString("usuario"),
		advplrt.NewString("senha"),
	}
	if err := v.callDbConnectionMethod(obj, "NEW", args); err != nil {
		t.Fatalf("NEW: %v", err)
	}
	st := obj.Native.(*dbConnState)
	if st.driver != "POSTGRES" || st.host != "10.0.0.5" || st.port != 5432 || st.service != "meubanco" {
		t.Fatalf("unexpected state after NEW: %+v", st)
	}
}

func TestDbConnectionConnectUnknownDriverSetsError(t *testing.T) {
	resetDbaccessState()
	v := NewVM(&compiler.Bytecode{}, false)
	obj := newDbConnectionObject()
	_ = v.callDbConnectionMethod(obj, "NEW", []advplrt.Value{
		advplrt.NewString("DB2"), advplrt.NewString("host"), advplrt.NewNumber(1),
		advplrt.NewString("svc"), advplrt.NewString("u"), advplrt.NewString("p"),
	})
	if err := v.callDbConnectionMethod(obj, "CONNECT", nil); err != nil {
		t.Fatalf("CONNECT should not return a Go error, only push .F. and set GetError: %v", err)
	}
	ret := v.pop()
	if advplrt.ToBool(ret) {
		t.Fatal("Connect() with unknown driver should return .F.")
	}
	st := obj.Native.(*dbConnState)
	if st.lastError == "" {
		t.Fatal("GetError() state should be populated after a failed Connect()")
	}
}
