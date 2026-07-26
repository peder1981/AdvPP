// pkg/p2p/message.go
package p2p

import (
	"encoding/json"
)

type Message struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
	ID   string `json:"id,omitempty"`
}

func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func (m *Message) Unmarshal(data []byte) error {
	return json.Unmarshal(data, m)
}
