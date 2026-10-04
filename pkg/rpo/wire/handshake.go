package wire

import (
	"context"
	"fmt"
	"net"
	"time"
)

// HandshakeConfig parametriza o handshake de autenticação.
type HandshakeConfig struct {
	Env        string // ex.: "P12"
	User       string
	Pass       string
	Host       string // hostname local (campo informativo do banner)
	BuildStamp string // default BuildStamp
}

// HandshakeInfo é o resultado parseado do handshake.
type HandshakeInfo struct {
	Version   string // reply 0xa22b: "20.3.2.14 - 42368"
	Build     string // reply 0x8451: "7.00.210324P"
	EnvCode   uint32 // reply 0xa241: 1
	LoginOK   bool   // reply 0x0051: primeiro byte 0x01
	EnvMarker string // reply 0xa1c4: "TOTVSTEC"
}

type hsStep struct {
	msgID uint16
	body  []byte
}

// Handshake executa banner + as 5 mensagens de autenticação observadas ao
// vivo (capturas L334). A senha vai somente no frame 0x0051 e nunca aparece
// em erro ou log desta função (Lei 6).
func Handshake(ctx context.Context, conn net.Conn, cfg HandshakeConfig) (*HandshakeInfo, error) {
	if cfg.BuildStamp == "" {
		cfg.BuildStamp = BuildStamp
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	}
	defer func() { _ = conn.SetDeadline(time.Time{}) }()

	banner, err := BuildBanner(cfg.User, cfg.Host, cfg.BuildStamp)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(banner); err != nil {
		return nil, fmt.Errorf("enviando banner: %w", err)
	}

	// Descarta o ack do banner (janela de 1,5s — se não vier, segue).
	_ = conn.SetReadDeadline(time.Now().Add(1500 * time.Millisecond))
	ack := make([]byte, 4096)
	_, _ = conn.Read(ack)
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))

	env := cfg.Env
	steps := []hsStep{
		{0xa22b, nil},
		{0x8451, nil},
		{0xa241, []byte(env + "\x00")},
		{0x0051, []byte(env + "\x00" + cfg.User + "\x00" + cfg.Pass + "\x00")},
		{0xa1c4, nil},
	}
	info := &HandshakeInfo{}
	for i, st := range steps {
		if _, err := conn.Write(EncodeFrame(st.msgID, st.body)); err != nil {
			return nil, fmt.Errorf("handshake passo %d: %w", i+1, err)
		}
		frame, err := readOneFrame(conn)
		if err != nil {
			return nil, fmt.Errorf("handshake passo %d: %w", i+1, err)
		}
		data, _, err := DecodeBody(frame.Body)
		if err != nil {
			return nil, fmt.Errorf("handshake passo %d: %w", i+1, err)
		}
		switch i {
		case 0:
			info.Version = cstring(data)
		case 1:
			info.Build = cstring(data)
		case 2:
			if len(data) >= 4 {
				info.EnvCode = uint32(data[0]) | uint32(data[1])<<8 |
					uint32(data[2])<<16 | uint32(data[3])<<24
			}
		case 3:
			info.LoginOK = len(data) > 0 && data[0] == 0x01
			if !info.LoginOK {
				// sem ecoar corpo (pode conter credenciais)
				return nil, fmt.Errorf("login recusado pelo appserver (passo 4)")
			}
		case 4:
			info.EnvMarker = cstring(data)
		}
	}
	return info, nil
}

// readOneFrame lê do conn até completar ao menos um frame.
func readOneFrame(conn net.Conn) (Frame, error) {
	var acc []byte
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		if err != nil {
			return Frame{}, err
		}
		acc = append(acc, tmp[:n]...)
		frames, _, perr := ParseFrames(acc)
		if perr != nil {
			return Frame{}, perr
		}
		if len(frames) > 0 {
			return frames[0], nil
		}
	}
}

// cstring corta no primeiro NUL.
func cstring(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
