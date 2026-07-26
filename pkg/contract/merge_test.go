package contract

import (
	"testing"
)

func TestMaxMonoidMerge(t *testing.T) {
	m := &MaxMonoid{}

	s1 := []byte("5")
	s2 := []byte("3")
	s3 := []byte("7")

	result := m.Merge(s1, s2)
	if string(result) != "5" {
		t.Errorf("expected 5, got %s", string(result))
	}

	result = m.Merge(result, s3)
	if string(result) != "7" {
		t.Errorf("expected 7, got %s", string(result))
	}

	result = m.Merge(s1, s1)
	if string(result) != "5" {
		t.Errorf("idempotence failed: %s", string(result))
	}
}

func TestMergeCommutativity(t *testing.T) {
	m := &MaxMonoid{}

	s1 := []byte("10")
	s2 := []byte("20")

	ab := m.Merge(s1, s2)
	ba := m.Merge(s2, s1)

	if string(ab) != string(ba) {
		t.Errorf("commutativity failed: %s vs %s", string(ab), string(ba))
	}
}
