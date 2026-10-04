package wire

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"
)

// goldenEmptyA22B é o frame 0xa22b vazio capturado ao vivo
// (captures2/285523_L334_C2S.bin — 18 bytes, sem segredo).
var goldenEmptyA22B = []byte{
	0x0e, 0x00, 0x00, 0x00, 0x21, 0xab, 0x0e, 0x00, 0x00, 0x00,
	0x04, 0x00, 0x00, 0x00, 0x01, 0x00, 0x2b, 0xa2,
}

func TestEncodeFrameVazioIgualCaptura(t *testing.T) {
	got := EncodeFrame(0xa22b, nil)
	if !bytes.Equal(got, goldenEmptyA22B) {
		t.Fatalf("encode diverge da captura ao vivo:\n got=%x\nwant=%x", got, goldenEmptyA22B)
	}
}

func TestEncodeParseRoundTrip(t *testing.T) {
	body := []byte("SourceName()\x00")
	buf := append(EncodeFrame(0x8149, body), EncodeFrame(0xa22b, nil)...)
	frames, rest, err := ParseFrames(buf)
	if err != nil {
		t.Fatal(err)
	}
	if rest != len(buf) {
		t.Fatalf("rest=%d, esperado %d", rest, len(buf))
	}
	if len(frames) != 2 {
		t.Fatalf("frames=%d, esperado 2", len(frames))
	}
	if frames[0].MsgID != 0x8149 || frames[0].Seq != 1 || !bytes.Equal(frames[0].Body, body) {
		t.Fatalf("frame 0: %+v", frames[0])
	}
	if frames[1].MsgID != 0xa22b || len(frames[1].Body) != 0 {
		t.Fatalf("frame 1: %+v", frames[1])
	}
}

func TestParseFramesIncompleto(t *testing.T) {
	full := EncodeFrame(0x8149, []byte("abc"))
	_, rest, err := ParseFrames(full[:len(full)-2])
	if err != nil {
		t.Fatal(err)
	}
	if rest != 0 {
		t.Fatalf("rest=%d, esperado 0 (frame incompleto)", rest)
	}
}

func TestParseFramesMagicInvalido(t *testing.T) {
	bad := append([]byte{}, goldenEmptyA22B...)
	bad[4] = 0x00
	if _, _, err := ParseFrames(bad); err == nil {
		t.Fatal("esperava erro de magic inválido")
	}
}

func TestDecodeBodyZlib(t *testing.T) {
	payload := []byte("conteudo-comprimido-da-resposta")
	var zb bytes.Buffer
	w := zlib.NewWriter(&zb)
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	w.Close()
	body := make([]byte, 8+zb.Len())
	binary.LittleEndian.PutUint32(body[0:], uint32(0x80000000|zb.Len()))
	binary.LittleEndian.PutUint32(body[4:], uint32(len(payload)))
	copy(body[8:], zb.Bytes())
	got, comp, err := DecodeBody(body)
	if err != nil || !comp {
		t.Fatalf("err=%v comp=%v", err, comp)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got=%q", got)
	}
}

func TestDecodeBodyPlano(t *testing.T) {
	in := []byte("texto simples")
	got, comp, err := DecodeBody(in)
	if err != nil || comp {
		t.Fatalf("err=%v comp=%v", err, comp)
	}
	if !bytes.Equal(got, in) {
		t.Fatal("plano deveria passar adiante")
	}
}
