package storage

import (
	"testing"
)

func TestStorePutGet(t *testing.T) {
	store := NewStore()

	key := "contract:abc123"
	data := []byte("state-data")

	err := store.Put(key, data)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	retrieved, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("data mismatch: %s vs %s", string(retrieved), string(data))
	}
}

func TestStoreDelete(t *testing.T) {
	store := NewStore()

	key := "contract:xyz"
	store.Put(key, []byte("data"))

	err := store.Delete(key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(key)
	if err == nil {
		t.Fatal("key should not exist after delete")
	}
}
