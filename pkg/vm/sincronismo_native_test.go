package vm

import (
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestGlbNmLock_Success(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Test basic lock acquisition
	got, err := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_TEST")})
	if err != nil {
		t.Fatalf("GlbNmLock retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("GlbNmLock(LOCK_TEST) = %v, quer .T. (lock adquirido)", got)
	}
}

func TestGlbNmLock_AlreadyLocked(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// First lock should succeed
	got1, err1 := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_TEST")})
	if err1 != nil {
		t.Fatalf("Primeira chamada GlbNmLock retornou erro: %v", err1)
	}
	b1, ok1 := got1.(*advplrt.BoolValue)
	if !ok1 || !b1.Val {
		t.Fatalf("Primeira chamada GlbNmLock(LOCK_TEST) = %v, quer .T.", got1)
	}

	// Second lock attempt should fail (already locked by same process)
	got2, err2 := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_TEST")})
	if err2 != nil {
		t.Fatalf("Segunda chamada GlbNmLock retornou erro: %v", err2)
	}
	b2, ok2 := got2.(*advplrt.BoolValue)
	if !ok2 || b2.Val {
		t.Errorf("Segunda chamada GlbNmLock(LOCK_TEST) = %v, quer .F. (já bloqueado)", got2)
	}
}

func TestGlbNmUnlock_Success(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Lock first
	_, err1 := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_TEST")})
	if err1 != nil {
		t.Fatalf("GlbNmLock retornou erro: %v", err1)
	}

	// Then unlock
	got, err := v.natives["GLBNMUNLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_TEST")})
	if err != nil {
		t.Fatalf("GlbNmUnlock retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("GlbNmUnlock(LOCK_TEST) = %v, quer .T. (desbloqueio bem-sucedido)", got)
	}
}

func TestGlbNmUnlock_NotLocked(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Try to unlock without locking first
	got, err := v.natives["GLBNMUNLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_NOTEXISTS")})
	if err != nil {
		t.Fatalf("GlbNmUnlock retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || b.Val {
		t.Errorf("GlbNmUnlock(LOCK_NOTEXISTS) = %v, quer .F. (lock não existe)", got)
	}
}

func TestGlbNmUnlock_AfterLockAndUnlock(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Lock
	_, err1 := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_TEST")})
	if err1 != nil {
		t.Fatalf("GlbNmLock retornou erro: %v", err1)
	}

	// Unlock
	_, err2 := v.natives["GLBNMUNLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_TEST")})
	if err2 != nil {
		t.Fatalf("Primeira GlbNmUnlock retornou erro: %v", err2)
	}

	// Try to unlock again (should fail)
	got, err := v.natives["GLBNMUNLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_TEST")})
	if err != nil {
		t.Fatalf("Segunda GlbNmUnlock retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || b.Val {
		t.Errorf("Segunda GlbNmUnlock(LOCK_TEST) = %v, quer .F. (já desbloqueado)", got)
	}
}

func TestGlbNmLock_MultipleIndependentLocks(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Lock multiple independent locks
	got1, err1 := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_A")})
	if err1 != nil {
		t.Fatalf("GlbNmLock(LOCK_A) retornou erro: %v", err1)
	}
	b1, ok1 := got1.(*advplrt.BoolValue)
	if !ok1 || !b1.Val {
		t.Errorf("GlbNmLock(LOCK_A) = %v, quer .T.", got1)
	}

	got2, err2 := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_B")})
	if err2 != nil {
		t.Fatalf("GlbNmLock(LOCK_B) retornou erro: %v", err2)
	}
	b2, ok2 := got2.(*advplrt.BoolValue)
	if !ok2 || !b2.Val {
		t.Errorf("GlbNmLock(LOCK_B) = %v, quer .T.", got2)
	}

	// Both locks should be in place
	// Verify by trying to lock them again (should fail)
	got3, err3 := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString("LOCK_A")})
	if err3 != nil {
		t.Fatalf("Segunda GlbNmLock(LOCK_A) retornou erro: %v", err3)
	}
	b3, ok3 := got3.(*advplrt.BoolValue)
	if !ok3 || b3.Val {
		t.Errorf("Segunda GlbNmLock(LOCK_A) = %v, quer .F.", got3)
	}
}

func TestGlbNmLock_EmptyString(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Test with empty string (should still work)
	got, err := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString("")})
	if err != nil {
		t.Fatalf("GlbNmLock(\"\") retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("GlbNmLock(\"\") = %v, quer .T.", got)
	}
}

func TestGlbNmLock_LongIdentifier(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Test with a long identifier name
	longID := "VERY_LONG_LOCK_IDENTIFIER_NAME_FOR_TESTING_PURPOSES"
	got, err := v.natives["GLBNMLOCK"].Fn([]advplrt.Value{advplrt.NewString(longID)})
	if err != nil {
		t.Fatalf("GlbNmLock(long) retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("GlbNmLock(long) = %v, quer .T.", got)
	}
}
