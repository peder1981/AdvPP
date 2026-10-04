// Package wire implementa o codec do protocolo de aplicação
// AdvPL/AppServer observado ao vivo no laboratório (proxy tcp_proxy2,
// ondas 19–38): frame <IHIIHH> + body, magic 0xab21, envelope zlib
// opcional no body das respostas grandes.
package wire

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	frameHeaderLen = 18
	frameMagic     = 0xab21
)

// Frame é um frame completo do protocolo.
type Frame struct {
	MsgID uint16
	Seq   uint16
	Body  []byte
}

// EncodeFrame monta um frame (seq=1 — único valor observado em todas as
// capturas ao vivo). Layout: u32 total-4, u16 magic, u32 total-4,
// u32 total-14, u16 seq, u16 msgID, body.
func EncodeFrame(msgID uint16, body []byte) []byte {
	total := frameHeaderLen + len(body)
	buf := make([]byte, total)
	le := binary.LittleEndian
	le.PutUint32(buf[0:], uint32(total-4))
	le.PutUint16(buf[4:], frameMagic)
	le.PutUint32(buf[6:], uint32(total-4))
	le.PutUint32(buf[10:], uint32(total-14))
	le.PutUint16(buf[14:], 1)
	le.PutUint16(buf[16:], msgID)
	copy(buf[frameHeaderLen:], body)
	return buf
}

// ParseFrames decodifica os frames completos de buf. rest é o offset onde
// começam os bytes restantes (frame incompleto). Erro se o magic não bater.
func ParseFrames(buf []byte) (frames []Frame, rest int, err error) {
	le := binary.LittleEndian
	off := 0
	for off+frameHeaderLen <= len(buf) {
		if le.Uint16(buf[off+4:]) != frameMagic {
			return frames, off, fmt.Errorf("magic inválido no offset %d: %#x",
				off, le.Uint16(buf[off+4:]))
		}
		total := int(le.Uint32(buf[off:])) + 4
		if total < frameHeaderLen {
			return frames, off, fmt.Errorf("total inválido (%d) no offset %d", total, off)
		}
		if off+total > len(buf) {
			break
		}
		frames = append(frames, Frame{
			MsgID: le.Uint16(buf[off+16:]),
			Seq:   le.Uint16(buf[off+14:]),
			Body:  buf[off+frameHeaderLen : off+total],
		})
		off += total
	}
	return frames, off, nil
}

// DecodeBody remove o envelope zlib quando presente (bit 0x80 do primeiro
// u32; depois vêm u32 compLen e u32 uncompLen). Retorna os dados puros.
func DecodeBody(body []byte) (data []byte, compressed bool, err error) {
	if len(body) < 8 {
		return body, false, nil
	}
	le := binary.LittleEndian
	raw := le.Uint32(body[0:])
	if raw&0x80000000 == 0 {
		return body, false, nil
	}
	compLen := int(raw & 0x7fffffff)
	if 8+compLen > len(body) {
		return nil, true, fmt.Errorf("compLen %d excede body (%d)", compLen, len(body))
	}
	r, err := zlib.NewReader(bytes.NewReader(body[8 : 8+compLen]))
	if err != nil {
		return nil, true, fmt.Errorf("zlib: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, true, fmt.Errorf("zlib read: %w", err)
	}
	return out, true, nil
}
