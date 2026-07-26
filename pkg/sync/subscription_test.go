package sync

import (
	"testing"
	"time"
)

func TestSubscribeToContract(t *testing.T) {
	tree := NewSubscriptionTree("contract-key")

	_ = tree.Subscribe("peer-1")

	if !tree.IsSubscribed("peer-1") {
		t.Fatal("peer not subscribed")
	}

	if len(tree.GetSubscribers()) != 1 {
		t.Errorf("expected 1 subscriber, got %d", len(tree.GetSubscribers()))
	}
}

func TestSubscriptionLeaseExpiry(t *testing.T) {
	tree := NewSubscriptionTree("contract-key")

	sub := tree.Subscribe("peer-1")

	sub.ExpiresAt = time.Now().Add(-time.Second)

	if tree.IsSubscribed("peer-1") {
		t.Fatal("subscription should be expired")
	}

	tree.ClearExpired()

	if tree.IsSubscribed("peer-1") {
		t.Fatal("expired subscription not cleared")
	}
}
