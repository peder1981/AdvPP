package wire

import "fmt"

// Layout do banner do cliente (134 bytes), medido no capture ao vivo
// captures2/0222_L7_C2S.bin (onda 7):
//
//	[0:15]   "--ADVANCEDPR--\x00"
//	[15]     0x03
//	[16:82]  user   (65 chars + NUL, zero-pad)
//	[82:115] host   (32 chars + NUL, zero-pad)
//	[115:130] build (14 chars + NUL, zero-pad)
//	[130:134] "\x00\x00\x05\x01"
const (
	bannerLen    = 134
	bannerUserAt = 16
	bannerUserSz = 66
	bannerHostAt = 82
	bannerHostSz = 33
	bannerBldAt  = 115
	bannerBldSz  = 15
)

// BuildStamp é o campo data/hora do build observado no banner da DA que
// autentica com sucesso neste appserver (build 7.00.210324P).
const BuildStamp = "20210324103317"

// BuildBanner monta o banner de identificação do cliente.
// Falha se algum campo não couber (fail fast — sem truncamento silencioso).
func BuildBanner(user, host, buildStamp string) ([]byte, error) {
	if len(user) >= bannerUserSz {
		return nil, fmt.Errorf("user não cabe no banner (%d ≥ %d)", len(user), bannerUserSz)
	}
	if len(host) >= bannerHostSz {
		return nil, fmt.Errorf("host não cabe no banner (%d ≥ %d)", len(host), bannerHostSz)
	}
	if len(buildStamp) >= bannerBldSz {
		return nil, fmt.Errorf("build não cabe no banner (%d ≥ %d)", len(buildStamp), bannerBldSz)
	}
	b := make([]byte, bannerLen) // make zera → campos já saem zero-pad
	copy(b[0:15], "--ADVANCEDPR--\x00")
	b[15] = 0x03
	copy(b[bannerUserAt:], user)
	copy(b[bannerHostAt:], host)
	copy(b[bannerBldAt:], buildStamp)
	copy(b[130:], []byte{0x00, 0x00, 0x05, 0x01})
	return b, nil
}
