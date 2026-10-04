package wire

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func getenv(k string) string      { return os.Getenv(k) }
func osHostname() (string, error) { return os.Hostname() }

// fakeHSServer simula o appserver: lê banner, lê 5 frames, valida e responde
// com os corpos capturados ao vivo. loginOK controla o frame 0x0051.
func fakeHSServer(t *testing.T, conn net.Conn, cfg HandshakeConfig, loginOK bool) <-chan error {
	t.Helper()
	errc := make(chan error, 1)
	go func() {
		defer close(errc)
		banner, err := BuildBanner(cfg.User, cfg.Host, cfg.BuildStamp)
		if err != nil {
			errc <- err
			return
		}
		buf := make([]byte, len(banner))
		if _, err := io.ReadFull(conn, buf); err != nil {
			errc <- err
			return
		}
		// ack do banner (o servidor manda algo antes das respostas)
		if _, err := conn.Write(EncodeFrame(0xc16e, []byte("ack\x00"))); err != nil {
			errc <- err
			return
		}
		want := []struct {
			mid  uint16
			body string
		}{
			{0xa22b, ""},
			{0x8451, ""},
			{0xa241, cfg.Env + "\x00"},
			{0x0051, cfg.Env + "\x00" + cfg.User + "\x00" + cfg.Pass + "\x00"},
			{0xa1c4, ""},
		}
		replies := [][]byte{
			[]byte("20.3.2.14 - 42368\x00"),
			[]byte("7.00.210324P\x00"),
			{0x01, 0x00, 0x00, 0x00},
			{0x01, 'N', '0', 0x00},
			[]byte("TOTVSTEC\x00"),
		}
		if !loginOK {
			replies[3] = []byte{0x00, 'E', 'r', 'r', 0x00}
		}
		var acc []byte
		tmp := make([]byte, 4096)
		for i, w := range want {
			// lê até completar 1 frame
			var frames []Frame
			var rest int
			var perr error
			for {
				n, err := conn.Read(tmp)
				if err != nil {
					errc <- err
					return
				}
				acc = append(acc, tmp[:n]...)
				frames, rest, perr = ParseFrames(acc)
				if perr != nil {
					errc <- perr
					return
				}
				if len(frames) > 0 {
					acc = acc[rest:]
					break
				}
			}
			f := frames[0]
			if f.MsgID != w.mid {
				errc <- &stepErr{i, "msgID", f.MsgID, w.mid}
				return
			}
			if string(f.Body) != w.body {
				errc <- &stepErr{i, "body", string(f.Body), w.body}
				return
			}
			if _, err := conn.Write(EncodeFrame(0xc16e, replies[i])); err != nil {
				errc <- err
				return
			}
			// recusa de login: o cliente aborta aqui (não manda o passo 5)
			if i == 3 && !loginOK {
				errc <- nil
				return
			}
		}
		errc <- nil
	}()
	return errc
}

type stepErr struct {
	step      int
	field     string
	got, want any
}

func (e *stepErr) Error() string {
	return fmt.Sprintf("passo %d %s: got=%v want=%v", e.step+1, e.field, e.got, e.want)
}

func TestHandshakeSucesso(t *testing.T) {
	cfg := HandshakeConfig{Env: "P12", User: "u1", Pass: "dummy-pass",
		Host: "h1", BuildStamp: BuildStamp}
	c1, c2 := net.Pipe()
	defer c1.Close()
	errc := fakeHSServer(t, c2, cfg, true)
	defer c2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, err := Handshake(ctx, c1, cfg)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("servidor fake: %v", err)
	}
	if !info.LoginOK || info.EnvCode != 1 {
		t.Fatalf("info: %+v", info)
	}
	if info.Version != "20.3.2.14 - 42368" || info.Build != "7.00.210324P" ||
		info.EnvMarker != "TOTVSTEC" {
		t.Fatalf("campos: %+v", info)
	}
}

func TestHandshakeLoginRecusado(t *testing.T) {
	cfg := HandshakeConfig{Env: "P12", User: "u1", Pass: "dummy-pass",
		Host: "h1", BuildStamp: BuildStamp}
	c1, c2 := net.Pipe()
	defer c1.Close()
	errc := fakeHSServer(t, c2, cfg, false)
	defer c2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := Handshake(ctx, c1, cfg)
	if err == nil {
		t.Fatal("esperava erro de login")
	}
	// Lei 6: a senha nunca pode aparecer em mensagem de erro.
	if strings.Contains(err.Error(), cfg.Pass) {
		t.Fatalf("erro contém a senha: %v", err)
	}
	if e := <-errc; e != nil {
		t.Fatalf("servidor fake: %v", e)
	}
}

// TestHandshakeLive valida contra o appserver real do laboratório.
// Requer ADVPP_RPO_LIVE=1 e ADVPP_RPO_PASS no ambiente.
func TestHandshakeLive(t *testing.T) {
	pass := getenv("ADVPP_RPO_PASS")
	if getenv("ADVPP_RPO_LIVE") == "" || pass == "" {
		t.Skip("requer ADVPP_RPO_LIVE=1 e ADVPP_RPO_PASS")
	}
	host, err := osHostname()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", "172.17.0.3:1234", 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	info, err := Handshake(ctx, conn, HandshakeConfig{
		Env: "P12", User: "peder", Pass: pass, Host: host, BuildStamp: BuildStamp,
	})
	if err != nil {
		t.Fatalf("handshake live: %v", err)
	}
	if !info.LoginOK {
		t.Fatal("login não ok no appserver real")
	}
	t.Logf("live ok: build=%s env=%s versao=%s", info.Build, info.EnvMarker, info.Version)
}
