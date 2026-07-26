package p2p

import (
	"testing"
	"github.com/advpl/compiler/pkg/contract"
	"github.com/advpl/compiler/pkg/storage"
)

func TestPutAndGet(t *testing.T) {
	store := storage.NewStore()
	api := NewPeerAPI(store)

	contractKey := "test-contract"
	data := []byte("state-value")

	err := api.Put(contractKey, data)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	retrieved, err := api.Get(contractKey)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved) != string(data) {
		t.Errorf("data mismatch: %s vs %s", string(retrieved), string(data))
	}
}

func TestUpdate(t *testing.T) {
	store := storage.NewStore()
	merge := &contract.MaxMonoid{}
	api := NewPeerAPI(store)

	contractKey := "test-contract"

	api.Put(contractKey, []byte("10"))

	err := api.Update(contractKey, []byte("5"), merge)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	result, _ := api.Get(contractKey)
	if string(result) != "10" {
		t.Errorf("expected max(10, 5) = 10, got %s", string(result))
	}
}
