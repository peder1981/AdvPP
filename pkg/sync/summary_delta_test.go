package sync

import (
	"testing"
	"github.com/advpl/compiler/pkg/contract"
)

func TestSummarizeDelta(t *testing.T) {
	c := &contract.Contract{
		Key: "test-contract",
	}

	sync := NewSummaryDeltaSync(c, &contract.MaxMonoid{})

	stateA := []byte("10")

	summary := sync.Summarize(stateA)

	if len(summary) == 0 {
		t.Fatal("summary is empty")
	}

	stateB := []byte("5")

	delta := sync.GetDelta(stateB, summary)

	if len(delta) == 0 {
		t.Fatal("delta is empty")
	}

	result := sync.ApplyDelta(stateB, delta)

	if string(result) != "10" {
		t.Errorf("expected 10, got %s", string(result))
	}
}
