package contract

import (
	"testing"
)

func TestNewContractDeterministic(t *testing.T) {
	code := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	params := []byte("test-params")

	c1 := NewContract(code, params)
	c2 := NewContract(code, params)

	if c1.Location != c2.Location {
		t.Errorf("Location not deterministic: %f vs %f", c1.Location, c2.Location)
	}
	if c1.Key != c2.Key {
		t.Errorf("Key not deterministic: %s vs %s", c1.Key, c2.Key)
	}
	if c1.CodeHash != c2.CodeHash {
		t.Errorf("CodeHash not deterministic: %s vs %s", c1.CodeHash, c2.CodeHash)
	}
}

func TestLoadAndRetrieveContract(t *testing.T) {
	code := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	c := NewContract(code, []byte{})

	runtime := NewContractRuntime()
	err := runtime.LoadContract(c)
	if err != nil {
		t.Fatalf("LoadContract failed: %v", err)
	}

	retrieved := runtime.GetContract(c.Key)
	if retrieved == nil {
		t.Fatal("Contract not found after loading")
	}
	if retrieved.Key != c.Key {
		t.Errorf("Retrieved contract key mismatch: %s vs %s", retrieved.Key, c.Key)
	}
}
