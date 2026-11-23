package main

// MFA TOTP (RFC 6238) dos endpoints /api/framework/mfa/* consumidos pelo
// @totvs/protheus-lib-core. O Protheus real guarda o segredo no servidor;
// aqui persistimos em ~/.advpp/mfa_secrets.json por usuário, com flag
// "confirmed" — o segredo é gerado pelo TOTPURI (enrollment via QR) e só
// passa a valer (hasSecret=true) após o primeiro código válido, espelhando
// o fluxo de confirmação do autenticador.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/advpl/compiler/pkg/rest"
)

type mfaSegredo struct {
	Secret    string `json:"secret"`    // base32 sem padding
	Confirmed bool   `json:"confirmed"` // true após primeiro tok válido
}

type mfaStore struct {
	mu      sync.Mutex
	path    string
	secrets map[string]*mfaSegredo
}

var mfa = &mfaStore{secrets: map[string]*mfaSegredo{}}

func mfaPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".advpp", "mfa_secrets.json")
}

// mfaLoad carrega o store uma única vez (lazy).
func (s *mfaStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path != "" {
		return
	}
	s.path = mfaPath()
	if s.path == "" {
		return
	}
	if b, err := os.ReadFile(s.path); err == nil {
		var m map[string]*mfaSegredo
		if json.Unmarshal(b, &m) == nil {
			s.secrets = m
		}
	}
}

func (s *mfaStore) save() {
	if s.path == "" {
		return
	}
	b, err := json.MarshalIndent(s.secrets, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0755)
	_ = os.WriteFile(s.path, b, 0600)
}

// segredoDo devolve o registro do usuário (nil se nunca gerou URI).
func (s *mfaStore) segredoDo(user string) *mfaSegredo {
	s.load()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.secrets[user]
}

// segredoOuGera devolve o segredo base32 do usuário, gerando um novo
// (20 bytes aleatórios — padrão TOTP) quando ainda não existe.
func (s *mfaStore) segredoOuGera(user string) string {
	s.load()
	s.mu.Lock()
	defer s.mu.Unlock()
	if sg := s.secrets[user]; sg != nil && sg.Secret != "" {
		return sg.Secret
	}
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	s.secrets[user] = &mfaSegredo{Secret: secret}
	s.save()
	return secret
}

// temSegredoConfirmed: hasSecret do endpoint — só true após confirmação.
func (s *mfaStore) temSegredoConfirmed(user string) bool {
	sg := s.segredoDo(user)
	return sg != nil && sg.Confirmed
}

// validar confere um código TOTP de 6 dígitos na janela ±1 período
// (tolera drift de relógio). No primeiro acerto com segredo ainda não
// confirmado, confirma o enrollment (fluxo "escaneou o QR → digitou o
// código → MFA ativado").
func (s *mfaStore) validar(user, codigo string) bool {
	s.load()
	s.mu.Lock()
	defer s.mu.Unlock()
	sg := s.secrets[user]
	if sg == nil || sg.Secret == "" {
		return false
	}
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(sg.Secret)
	if err != nil {
		return false
	}
	codigo = strings.TrimSpace(codigo)
	if len(codigo) != 6 {
		return false
	}
	n, err := strconv.Atoi(codigo)
	if err != nil {
		return false
	}
	agora := time.Now().Unix() / 30
	for delta := int64(-1); delta <= 1; delta++ {
		if totpCode(raw, uint64(int64(agora)+delta)) == n {
			if !sg.Confirmed {
				sg.Confirmed = true
				s.save()
			}
			return true
		}
	}
	return false
}

// totpCode implementa RFC 6238 (HMAC-SHA1, 8 bytes da mensagem, dynamic
// truncation, 6 dígitos).
func totpCode(secret []byte, counter uint64) int {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := int((uint32(sum[off])&0x7f)<<24 | uint32(sum[off+1])<<16 | uint32(sum[off+2])<<8 | uint32(sum[off+3]))
	return code % 1000000
}

// uriOTP gera o otpauth:// que o app converte em QR code
// (ProMfaQrCodeInterface.cQrCode).
func uriOTP(user, secret string) string {
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", "AdvPP")
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape("AdvPP:"+user) + "?" + q.Encode()
}

// handlerMFAHasSecret — GET /api/framework/mfa/hasSecret → {hasSecret}.
func handlerMFAHasSecret(r *rest.Request) *rest.Response {
	_, _, _, usuario := frameworkSessao()
	return fwJSON(map[string]any{"hasSecret": mfa.temSegredoConfirmed(usuario)})
}

// handlerMFATOTPURI — GET /api/framework/mfa/TOTPURI → {cQrCode}.
// Gera (ou reutiliza) o segredo do usuário e devolve o otpauth:// para o
// app montar o QR de enrollment.
func handlerMFATOTPURI(r *rest.Request) *rest.Response {
	_, _, _, usuario := frameworkSessao()
	secret := mfa.segredoOuGera(usuario)
	if secret == "" {
		return fwErro(500, "falha ao gerar segredo MFA")
	}
	return fwJSON(map[string]any{"cQrCode": uriOTP(usuario, secret)})
}

// handlerMFATok — GET /api/framework/mfa/tok?token=CODIGO → {tok}.
// tok=1 quando o TOTP confere (e confirma o enrollment no primeiro uso),
// 0 caso contrário — a lib usa o valor como flag de token válido.
func handlerMFATok(r *rest.Request) *rest.Response {
	_, _, _, usuario := frameworkSessao()
	codigo := r.Query.Get("token")
	if mfa.validar(usuario, codigo) {
		return fwJSON(map[string]any{"tok": 1})
	}
	return fwJSON(map[string]any{"tok": 0})
}
