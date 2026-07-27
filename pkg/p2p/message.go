// pkg/p2p/message.go
package p2p

import (
	"encoding/json"
)

type Message struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
	ID   string `json:"id,omitempty"`

	// Key is the contract key an UPDATE/STATE_SYNC message applies to.
	// Empty for message types that don't carry contract state (e.g. JOIN).
	Key string `json:"key,omitempty"`
	// From is the sender's own "host:port" listen address, self-reported
	// (UDP delivery doesn't expose it to the receiver's read loop). Used by
	// the chat bridge (pkg/vm) to reply to JOIN with a direct STATE_SYNC and
	// to register the sender as a ring neighbor.
	From string `json:"from,omitempty"`
}

func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Message) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}
