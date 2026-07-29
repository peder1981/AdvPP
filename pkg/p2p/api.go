package p2p

import (
	"github.com/advpl/compiler/pkg/contract"
	"github.com/advpl/compiler/pkg/storage"
)

// PeerAPI provides contract operations (Get, Put, Update) with merge semantics.
type PeerAPI struct {
	store *storage.Store
}

// NewPeerAPI creates a new peer API backed by the given store.
func NewPeerAPI(store *storage.Store) *PeerAPI {
	return &PeerAPI{
		store: store,
	}
}

// Put stores a raw contract value.
func (api *PeerAPI) Put(contractKey string, data []byte) error {
	return api.store.Put(contractKey, data)
}

// Get retrieves a contract value by key.
func (api *PeerAPI) Get(contractKey string) ([]byte, error) {
	return api.store.Get(contractKey)
}

func (api *PeerAPI) Update(contractKey string, updateData []byte, merge contract.MergeOp) error {
	current, err := api.store.Get(contractKey)
	if err != nil {
		current = merge.Identity()
	}

	merged := merge.Merge(current, updateData)

	return api.store.Put(contractKey, merged)
}
