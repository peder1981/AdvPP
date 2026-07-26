package sync

import (
	"github.com/advpl/compiler/pkg/contract"
)

type SummaryDeltaSync struct {
	contract *contract.Contract
	merge    contract.MergeOp
}

func NewSummaryDeltaSync(c *contract.Contract, merge contract.MergeOp) *SummaryDeltaSync {
	return &SummaryDeltaSync{
		contract: c,
		merge:    merge,
	}
}

func (s *SummaryDeltaSync) Summarize(state []byte) []byte {
	return state
}

func (s *SummaryDeltaSync) GetDelta(myState, peerSummary []byte) []byte {
	if string(myState) != string(peerSummary) {
		return peerSummary
	}
	return []byte{}
}

func (s *SummaryDeltaSync) ApplyDelta(myState, delta []byte) []byte {
	if len(delta) == 0 {
		return myState
	}

	return s.merge.Merge(myState, delta)
}
