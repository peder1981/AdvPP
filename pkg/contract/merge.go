package contract

import (
	"bytes"
	"strconv"
)

// MergeOp defines the merge semantics for a contract (idempotent, commutative, associative).
type MergeOp interface {
	Merge(a, b []byte) []byte
	Identity() []byte
}

// MaxMonoid is a merge operation that keeps the maximum value (for numeric counters).
type MaxMonoid struct{}

func (m *MaxMonoid) Merge(a, b []byte) []byte {
	aVal, _ := strconv.Atoi(string(bytes.TrimSpace(a)))
	bVal, _ := strconv.Atoi(string(bytes.TrimSpace(b)))

	if aVal >= bVal {
		return a
	}
	return b
}

func (m *MaxMonoid) Identity() []byte {
	return []byte("0")
}

// LastWriterWinsMonoid is a merge operation that keeps the lexicographically larger value (last write wins).
type LastWriterWinsMonoid struct{}

func (m *LastWriterWinsMonoid) Merge(a, b []byte) []byte {
	if bytes.Compare(a, b) >= 0 {
		return a
	}
	return b
}

func (m *LastWriterWinsMonoid) Identity() []byte {
	return []byte{}
}

// ObservedRemoveSetMonoid removed: requires tag-based union, not in MVP scope
