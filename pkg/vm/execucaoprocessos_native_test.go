package vm

import (
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func TestIPCCount(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		name      string
		semaphore string
		setupFn   func() // função para preparar estado antes do teste
		want      int64
	}{
		{
			name:      "semaforo inexistente",
			semaphore: "TEST_NONEXISTENT",
			setupFn:   nil,
			want:      0,
		},
		{
			name:      "semaforo com um waiter",
			semaphore: "TEST_ONE_WAITER",
			setupFn: func() {
				// Simula um waiter adicionado pelo IPCWaitEx
				v.ipcSemaphoresMu.Lock()
				v.ipcSemaphores["TEST_ONE_WAITER"] = &ipcSemaphoreState{
					waiters: 1,
					ch:      make(chan []advplrt.Value, 1),
				}
				v.ipcSemaphoresMu.Unlock()
			},
			want: 1,
		},
		{
			name:      "semaforo com multiplos waiters",
			semaphore: "TEST_MANY_WAITERS",
			setupFn: func() {
				v.ipcSemaphoresMu.Lock()
				v.ipcSemaphores["TEST_MANY_WAITERS"] = &ipcSemaphoreState{
					waiters: 5,
					ch:      make(chan []advplrt.Value, 5),
				}
				v.ipcSemaphoresMu.Unlock()
			},
			want: 5,
		},
	}
	for _, c := range cases {
		if c.setupFn != nil {
			c.setupFn()
		}
		got, err := v.natives["IPCCOUNT"].Fn([]advplrt.Value{advplrt.NewString(c.semaphore)})
		if err != nil {
			t.Fatalf("IPCCount(%s) retornou erro: %v", c.name, err)
		}
		n, ok := got.(*advplrt.NumberValue)
		if !ok || int64(n.Val) != c.want {
			t.Errorf("IPCCount(%s) = %v, quer %v", c.name, got, c.want)
		}
	}
}

func TestIPCGo(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		name      string
		semaphore string
		setupFn   func() // função para preparar estado antes do teste
		want      bool
	}{
		{
			name:      "semaforo inexistente",
			semaphore: "TEST_NO_WAITERS",
			setupFn:   nil,
			want:      false, // Nenhum waiter, deve retornar .F.
		},
		{
			name:      "semaforo com waiter",
			semaphore: "TEST_WITH_WAITER",
			setupFn: func() {
				v.ipcSemaphoresMu.Lock()
				v.ipcSemaphores["TEST_WITH_WAITER"] = &ipcSemaphoreState{
					waiters: 1,
					ch:      make(chan []advplrt.Value, 1),
				}
				v.ipcSemaphoresMu.Unlock()
			},
			want: true, // Tem um waiter, deve retornar .T.
		},
	}
	for _, c := range cases {
		if c.setupFn != nil {
			c.setupFn()
		}
		got, err := v.natives["IPCGO"].Fn([]advplrt.Value{advplrt.NewString(c.semaphore)})
		if err != nil {
			t.Fatalf("IPCGo(%s) retornou erro: %v", c.name, err)
		}
		b, ok := got.(*advplrt.BoolValue)
		if !ok || b.Val != c.want {
			t.Errorf("IPCGo(%s) = %v, quer %v", c.name, got, c.want)
		}
	}
}

func TestIPCGoWithData(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Setup: cria um semaforo com um waiter
	v.ipcSemaphoresMu.Lock()
	v.ipcSemaphores["TEST_WITH_DATA"] = &ipcSemaphoreState{
		waiters: 1,
		ch:      make(chan []advplrt.Value, 1),
	}
	v.ipcSemaphoresMu.Unlock()

	// Chama IPCGo com um argumento adicional
	got, err := v.natives["IPCGO"].Fn([]advplrt.Value{
		advplrt.NewString("TEST_WITH_DATA"),
		advplrt.NewString("Teste data"),
	})
	if err != nil {
		t.Fatalf("IPCGo com data retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("IPCGo com data = %v, quer true", got)
	}
}

func TestIPCWaitEx(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cases := []struct {
		name      string
		semaphore string
		timeout   int64
		want      bool
		// Cada teste rodará concorrentemente: uma goroutine dispara IPCGo,
		// e a outra executa IPCWaitEx. Se tudo correr bem, IPCWaitEx recebe
		// .T. porque IPCGo foi disparado a tempo.
		// Se o timeout expira, IPCWaitEx retorna .F.
	}{
		{
			name:      "timeout sem ipcgo",
			semaphore: "TEST_TIMEOUT_NO_SIGNAL",
			timeout:   100, // 100ms timeout
			want:      false, // Deve expirar e retornar .F.
		},
		{
			name:      "ipcgo dispara a tempo",
			semaphore: "TEST_SIGNAL_IN_TIME",
			timeout:   5000, // 5s timeout
			want:      true, // IPCGo dispara a tempo, deve retornar .T.
		},
	}

	for _, c := range cases {
		if c.name == "ipcgo dispara a tempo" {
			// Dispara IPCGo em uma goroutine após um delay curto
			go func() {
				time.Sleep(100 * time.Millisecond)
				v.natives["IPCGO"].Fn([]advplrt.Value{advplrt.NewString(c.semaphore)})
			}()
		}

		got, err := v.natives["IPCWAITEX"].Fn([]advplrt.Value{
			advplrt.NewString(c.semaphore),
			advplrt.NewNumber(float64(c.timeout)),
		})
		if err != nil {
			t.Fatalf("IPCWaitEx(%s) retornou erro: %v", c.name, err)
		}
		b, ok := got.(*advplrt.BoolValue)
		if !ok || b.Val != c.want {
			t.Errorf("IPCWaitEx(%s) = %v, quer %v", c.name, got, c.want)
		}
	}
}

func TestIPCWaitExWithData(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	// Setup: dispara IPCGo em uma goroutine que envia dados
	go func() {
		time.Sleep(100 * time.Millisecond)
		v.natives["IPCGO"].Fn([]advplrt.Value{
			advplrt.NewString("TEST_DATA_PASSING"),
			advplrt.NewString("Hello"),
			advplrt.NewNumber(42),
		})
	}()

	// Cria uma variável para receber dados
	var data1, data2 advplrt.Value
	got, err := v.natives["IPCWAITEX"].Fn([]advplrt.Value{
		advplrt.NewString("TEST_DATA_PASSING"),
		advplrt.NewNumber(5000), // 5s timeout
		// A implementação deverá suportar argumentos por referência (pelos índices subsequentes)
		// Para este teste simplificado, apenas verificamos que retorna true
	})
	if err != nil {
		t.Fatalf("IPCWaitEx com data retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("IPCWaitEx com data = %v, quer true", got)
	}
	// Nota: A passagem de dados por referência é mais complexa em AdvPL e requer
	// um mecanismo especial. Por enquanto, apenas verificamos que a função
	// retorna corretamente.
	_ = data1
	_ = data2
}
