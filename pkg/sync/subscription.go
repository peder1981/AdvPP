package sync

import (
	"sync"
	"time"
)

type Subscription struct {
	PeerID    string
	StartedAt time.Time
	ExpiresAt time.Time
}

type SubscriptionTree struct {
	mu            sync.RWMutex
	contractKey   string
	subscriptions map[string]*Subscription
}

func NewSubscriptionTree(contractKey string) *SubscriptionTree {
	return &SubscriptionTree{
		contractKey:   contractKey,
		subscriptions: make(map[string]*Subscription),
	}
}

func (st *SubscriptionTree) Subscribe(peerID string) *Subscription {
	st.mu.Lock()
	defer st.mu.Unlock()

	sub := &Subscription{
		PeerID:    peerID,
		StartedAt: time.Now(),
		ExpiresAt: time.Now().Add(8 * time.Minute),
	}

	st.subscriptions[peerID] = sub
	return sub
}

func (st *SubscriptionTree) IsSubscribed(peerID string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()

	sub, exists := st.subscriptions[peerID]
	if !exists {
		return false
	}

	return sub.ExpiresAt.After(time.Now())
}

func (st *SubscriptionTree) GetSubscribers() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var result []string
	now := time.Now()

	for peerID, sub := range st.subscriptions {
		if sub.ExpiresAt.After(now) {
			result = append(result, peerID)
		}
	}

	return result
}

func (st *SubscriptionTree) ClearExpired() {
	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	for peerID, sub := range st.subscriptions {
		if sub.ExpiresAt.Before(now) {
			delete(st.subscriptions, peerID)
		}
	}
}

func (st *SubscriptionTree) RenewLease(peerID string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	sub, exists := st.subscriptions[peerID]
	if !exists {
		return nil
	}

	sub.ExpiresAt = time.Now().Add(8 * time.Minute)
	return nil
}
