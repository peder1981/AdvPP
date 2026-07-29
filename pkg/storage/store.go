package storage

import (
	"fmt"
	"sync"
)

// Store is a thread-safe in-memory key-value store for contract state.
type Store struct {
	mu    sync.RWMutex
	data  map[string][]byte
}

// NewStore creates a new empty in-memory key-value store.
func NewStore() *Store {
	return &Store{
		data: make(map[string][]byte),
	}
}

// Put stores a value under the given key, creating a copy to prevent external mutations.
func (s *Store) Put(key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	copied := make([]byte, len(data))
	copy(copied, data)

	s.data[key] = copied
	return nil
}

// Get retrieves a value by key, returning a copy to prevent external mutations.
func (s *Store) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, exists := s.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	copied := make([]byte, len(data))
	copy(copied, data)

	return copied, nil
}

// Delete removes a value by key (idempotent if key doesn't exist).
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// Keys returns all stored keys.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}
