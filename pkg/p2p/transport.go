// pkg/p2p/transport.go
package p2p

import (
	"fmt"
	"net"
)

// Transport handles UDP network communication for P2P messages.
type Transport struct {
	conn     *net.UDPConn
	Received chan *Message
}

// NewTransport performs a core operation.
func NewTransport(addr *net.UDPAddr) (*Transport, error) {
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen failed: %w", err)
	}

	return &Transport{
		conn:     conn,
		Received: make(chan *Message, 100),
	}, nil
}

func (t *Transport) LocalAddr() *net.UDPAddr {
	return t.conn.LocalAddr().(*net.UDPAddr)
}

func (t *Transport) Listen() {
	buf := make([]byte, 4096)
	for {
		n, _, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			break
		}

		var msg Message
		if err := msg.Unmarshal(buf[:n]); err != nil {
			continue
		}

		select {
		case t.Received <- &msg:
		default:
		}
	}
}

func (t *Transport) Send(remote *net.UDPAddr, msg *Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	_, err = t.conn.WriteToUDP(data, remote)
	return err
}

func (t *Transport) Close() error {
	return t.conn.Close()
}
