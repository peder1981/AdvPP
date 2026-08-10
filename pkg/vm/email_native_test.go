package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestGetMailObjReturnsNilWhenNotFound(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Try to get a mail object that was never set
	result, err := v.natives["GETMAILOBJ"].Fn([]advplrt.Value{advplrt.NewString("IMAPCONN")})
	if err != nil {
		t.Fatalf("GetMailObj retornou erro: %v", err)
	}
	if result != advplrt.Nil {
		t.Errorf("GetMailObj(\"IMAPCONN\") = %v, quer Nil (objeto não existente)", result)
	}
}

func TestSetMailObjStoresObjectAndRetrievesIt(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create a mock mail object (ObjectValue)
	mockMailObj := advplrt.NewObject("tMailManager", nil)

	// Store the object
	_, err := v.natives["SETMAILOBJ"].Fn([]advplrt.Value{
		advplrt.NewString("IMAPCONN"),
		mockMailObj,
	})
	if err != nil {
		t.Fatalf("SetMailObj retornou erro: %v", err)
	}

	// Retrieve the object
	result, err := v.natives["GETMAILOBJ"].Fn([]advplrt.Value{advplrt.NewString("IMAPCONN")})
	if err != nil {
		t.Fatalf("GetMailObj retornou erro: %v", err)
	}
	if result == advplrt.Nil {
		t.Errorf("GetMailObj(\"IMAPCONN\") retornou Nil, quer o objeto armazenado")
	}
	if result != mockMailObj {
		t.Errorf("GetMailObj(\"IMAPCONN\") retornou objeto diferente")
	}
}

func TestSetMailObjClearsObjectWhenNilPassed(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create and store a mock mail object
	mockMailObj := advplrt.NewObject("tMailManager", nil)
	_, err := v.natives["SETMAILOBJ"].Fn([]advplrt.Value{
		advplrt.NewString("IMAPCONN"),
		mockMailObj,
	})
	if err != nil {
		t.Fatalf("SetMailObj (store) retornou erro: %v", err)
	}

	// Verify it was stored
	result, _ := v.natives["GETMAILOBJ"].Fn([]advplrt.Value{advplrt.NewString("IMAPCONN")})
	if result == advplrt.Nil {
		t.Errorf("objeto não foi armazenado")
	}

	// Clear the object by passing Nil
	_, err = v.natives["SETMAILOBJ"].Fn([]advplrt.Value{
		advplrt.NewString("IMAPCONN"),
		advplrt.Nil,
	})
	if err != nil {
		t.Fatalf("SetMailObj (clear) retornou erro: %v", err)
	}

	// Verify it was cleared
	result, err = v.natives["GETMAILOBJ"].Fn([]advplrt.Value{advplrt.NewString("IMAPCONN")})
	if err != nil {
		t.Fatalf("GetMailObj retornou erro: %v", err)
	}
	if result != advplrt.Nil {
		t.Errorf("GetMailObj após SetMailObj(id, Nil) = %v, quer Nil", result)
	}
}

func TestMultipleMailObjectsByID(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	// Create and store multiple mail objects with different IDs
	mailObj1 := advplrt.NewObject("tMailManager", nil)
	mailObj2 := advplrt.NewObject("tMailManager", nil)

	_, err := v.natives["SETMAILOBJ"].Fn([]advplrt.Value{
		advplrt.NewString("CONN1"),
		mailObj1,
	})
	if err != nil {
		t.Fatalf("SetMailObj (CONN1) retornou erro: %v", err)
	}

	_, err = v.natives["SETMAILOBJ"].Fn([]advplrt.Value{
		advplrt.NewString("CONN2"),
		mailObj2,
	})
	if err != nil {
		t.Fatalf("SetMailObj (CONN2) retornou erro: %v", err)
	}

	// Verify both are stored independently
	result1, err := v.natives["GETMAILOBJ"].Fn([]advplrt.Value{advplrt.NewString("CONN1")})
	if err != nil || result1 != mailObj1 {
		t.Errorf("GetMailObj(\"CONN1\") falhou")
	}

	result2, err := v.natives["GETMAILOBJ"].Fn([]advplrt.Value{advplrt.NewString("CONN2")})
	if err != nil || result2 != mailObj2 {
		t.Errorf("GetMailObj(\"CONN2\") falhou")
	}
}

func TestSetMailObjReturnsNil(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)

	mockMailObj := advplrt.NewObject("tMailManager", nil)
	result, err := v.natives["SETMAILOBJ"].Fn([]advplrt.Value{
		advplrt.NewString("IMAPCONN"),
		mockMailObj,
	})
	if err != nil {
		t.Fatalf("SetMailObj retornou erro: %v", err)
	}
	if result != advplrt.Nil {
		t.Errorf("SetMailObj retornou %v, quer Nil", result)
	}
}
