package rpo

import (
	"fmt"
	"strings"
)

// RPOType identifica o tipo/variante de um RPO com base em magic e sentinela.
type RPOType int

const (
	RPOCustom   RPOType = iota // custom.rpo — APNSRM0419, sentinel 0xFFFFFFFF
	RPOTttm120                  // tttm120.rpo — APNSRM0421, sentinel 0xFFFFFF00
	RPOTlpp                     // tlpp.rpo — APNSRM0420, sentinel 0x0000FFFF
	RPOUnknown
)

// String retorna o nome legível do tipo.
func (t RPOType) String() string {
	switch t {
	case RPOCustom:
		return "custom"
	case RPOTttm120:
		return "tttm120"
	case RPOTlpp:
		return "tlpp"
	default:
		return "unknown"
	}
}

// RPOProfile descreve um RPO identificado com todas as suas características.
type RPOProfile struct {
	Type     RPOType
	Name     string
	Magic    string
	Sentinel uint32
	Size     int
}

// Identify analisa um buffer RPO e retorna o perfil identificado.
// A identificação é baseada em (magic, sentinel) — combinações únicas por build.
func Identify(data []byte) (*RPOProfile, error) {
	f, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return &RPOProfile{
		Type:     IdentifyType(f.FooterMagic, f.Sentinel),
		Name:     f.Name,
		Magic:    f.FooterMagic,
		Sentinel: f.Sentinel,
		Size:     len(data),
	}, nil
}

// IdentifyType determina o tipo a partir de magic + sentinel.
func IdentifyType(magic string, sentinel uint32) RPOType {
	switch {
	case magic == "APNSRM0419" && sentinel == 0xFFFFFFFF:
		return RPOCustom
	case magic == "APNSRM0421" && sentinel == 0xFFFFFF00:
		return RPOTttm120
	case magic == "APNSRM0420" && sentinel == 0x0000FFFF:
		return RPOTlpp
	default:
		return RPOUnknown
	}
}

// NeedsLiveExtraction indica se este RPO requer extração via appserver rodando.
func (p *RPOProfile) NeedsLiveExtraction() bool {
	return true
}

// SuggestExtractCommand retorna o comando sugerido para extrair funções
// deste RPO — o mesmo fluxo que `advplc rpo extract <rpo> --auto` executa
// automaticamente: colocar o RPO no diretório "apo" com o nome esperado,
// forçar um -compile de gatilho (que faz o appserver carregar e regravar o
// RPO existente) e interceptar tAppMap::EndBuild() via gdb.
func (p *RPOProfile) SuggestExtractCommand(rpoPath string) string {
	timeout := 90
	if p.Type == RPOTttm120 {
		timeout = 120 // ~379 MB, compilação de gatilho ainda é rápida mas o RPO em si é grande
	}
	return fmt.Sprintf(
		"# Modo automático (recomendado):\n"+
			"advplc rpo extract %s --auto\n"+
			"\n"+
			"# Equivalente manual (ajuste ADVPP_RPO_* se o container/ambiente for outro):\n"+
			"docker cp tools/rpo-live-inspect/extract_rpo.py protheus-compile:/tmp/extract_rpo.py\n"+
			"docker cp %s protheus-compile:/protheus12/apo/%s.rpo\n"+
			"docker exec protheus-compile bash -c 'printf \"User Function RPOEXT01()\\nReturn .T.\\n\" > /protheus12/apo/rpoextract_trigger.prw'\n"+
			"docker exec protheus-compile bash -c '\n"+
			"  cd /protheus12/bin/appserver && export LD_LIBRARY_PATH=.:$LD_LIBRARY_PATH &&\n"+
			"  timeout %d gdb -q -batch -x /tmp/extract_rpo.py --args ./appsrvlinux \\\n"+
			"    -compile -env=P12 -files=/protheus12/apo/rpoextract_trigger.prw -includes=/protheus12/apo\n"+
			"'\n"+
			"docker cp protheus-compile:/tmp/rpo_extract.log.json %s.extract.json\n"+
			"# Resultado: %s.extract.json",
		rpoPath, rpoPath, p.Name, timeout, rpoPath, rpoPath,
	)
}

// FormatSummary retorna um resumo legível do perfil.
func (p *RPOProfile) FormatSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Tipo ...........: %s\n", p.Type)
	fmt.Fprintf(&b, "Nome ...........: %s\n", p.Name)
	fmt.Fprintf(&b, "Magic ..........: %s\n", p.Magic)
	fmt.Fprintf(&b, "Sentinela ......: 0x%08X\n", p.Sentinel)
	fmt.Fprintf(&b, "Tamanho ........: %d bytes", p.Size)
	if p.Size >= 1024*1024 {
		fmt.Fprintf(&b, " (%.1f MB)", float64(p.Size)/(1024*1024))
	}
	b.WriteByte('\n')
	return b.String()
}
