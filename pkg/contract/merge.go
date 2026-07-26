package contract

import (
	"bytes"
	"strconv"
)

type MergeOp interface {
	Merge(a, b []byte) []byte
	Identity() []byte
}

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
