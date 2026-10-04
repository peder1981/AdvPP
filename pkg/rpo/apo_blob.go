package rpo

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ApoHeaderPrefix é o prefixo observado em TODOS os 3.465 blobs APO
// extraídos (onda 39): 75 00 00 46 46. Verificado por hexdump em
// EXTXDEF.PRW, TECA740A.PRW, ABSLOGGER.PRW e amostra TLPP (2026-10-04).
var ApoHeaderPrefix = []byte{0x75, 0x00, 0x00, 0x46, 0x46}

// ApoKind é o byte de tipo na posição 5 do blob.
type ApoKind byte

const (
	ApoKindAdvPL ApoKind = 'F' // PRW/PRX/PRG/APH/APW
	ApoKindTLPP  ApoKind = 'T' // TLPP
)

// String retorna a forma legível do kind.
func (k ApoKind) String() string {
	switch k {
	case ApoKindAdvPL:
		return "AdvPL"
	case ApoKindTLPP:
		return "TLPP"
	}
	return fmt.Sprintf("desconhecido(%q)", byte(k))
}

// ApoBlob é a desmontagem de um blob APO extraído via GetApoRes.
type ApoBlob struct {
	Raw      []byte
	Kind     ApoKind
	Size     int
	Strings  []ApoString
	FileName string

	Identifiers    []string
	Literals       []string
	Snippets       []string
	CallCandidates []CallCandidate
}

// CallCandidate é um candidato a chamada derivado por cruzamento com
// catálogo de nomes. InCatalog é FATO (consta no catálogo);
// Confidence é a relação de chamada — sempre INFERIDO (🟡), porque o
// blob APO não distingue uso de definição sem o layout completo.
type CallCandidate struct {
	Name       string `json:"name"`
	InCatalog  bool   `json:"in_catalog"`
	Confidence string `json:"confidence"`
}

// apoIdentRe: identificador AdvPL/TLPP clássico.
var apoIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{1,63}$`)

// apoSnippetRe: conteúdo característico de código AdvPL/TLPP dentro de
// string literal (padrões validados empiricamente contra os 3.465 blobs:
// 788 PRW / 723 TLPP / 22 PRX / 1 PRG contêm match — 2026-10-04).
var apoSnippetRe = regexp.MustCompile(`(?i):=|->|%notdel%|%xfilial|%exp:` +
	`|beginsql|endsql|select .* from |where |d_e_l_e_t_` +
	`|function |return |if\(|endif|while |for |dbselectarea` +
	`|reclock|msunlock|fwlogmsg|fwexecstatement`)

// Classify separa Strings em Identifiers / Literals / Snippets e cruza
// identificadores com o catálogo (oráculo) para gerar CallCandidates.
// Prioridade: snippet > identificador > literal; FileName é sempre
// excluído (é o próprio resource, não conteúdo).
func (b *ApoBlob) Classify(catalog map[string]bool) {
	b.Identifiers = nil
	b.Literals = nil
	b.Snippets = nil
	b.CallCandidates = nil
	for _, s := range b.Strings {
		if b.FileName != "" && s.Text == b.FileName {
			continue
		}
		switch {
		case apoSnippetRe.MatchString(s.Text):
			b.Snippets = append(b.Snippets, s.Text)
		case apoIdentRe.MatchString(s.Text):
			b.Identifiers = append(b.Identifiers, s.Text)
			if catalog != nil && catalog[strings.ToUpper(s.Text)] {
				b.CallCandidates = append(b.CallCandidates, CallCandidate{
					Name: s.Text, InCatalog: true, Confidence: "INFERIDO",
				})
			}
		default:
			b.Literals = append(b.Literals, s.Text)
		}
	}
}

// LoadApoCatalog carrega um arquivo com um nome por linha (ex.:
// wire/catalog_sec2.txt, 113.431 nomes) em map uppercase→true.
func LoadApoCatalog(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	catalog := make(map[string]bool, 128*1024)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		name := strings.TrimSpace(sc.Text())
		if name != "" {
			catalog[strings.ToUpper(name)] = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return catalog, nil
}

// ApoString é uma string imprimível localizada no blob.
type ApoString struct {
	Offset int    `json:"offset"`
	Text   string `json:"text"`
}

// apoFileExtRe reconhece o nome-do-recurso como string com extensão conhecida.
var apoFileExtRe = regexp.MustCompile(`(?i)^[A-Za-z0-9_.# -]+\.(PRW|TLPP|PRX|APH|APW|PRG|CH|TRES|TRP)$`)

// ExtractStrings varre o blob por runs imprimíveis (ASCII 32..126, >= 4
// chars) e deriva FileName = primeira string com extensão de resource
// conhecida (heurística verificada: filename único em fixtures "vazias"
// e presente perto do EOF em ABSLOGGER.PRW).
func (b *ApoBlob) ExtractStrings() {
	b.Strings = nil
	b.FileName = ""
	current := make([]byte, 0, 64)
	start := 0
	flush := func() {
		if len(current) >= 4 {
			text := string(current)
			b.Strings = append(b.Strings, ApoString{Offset: start, Text: text})
			if b.FileName == "" && apoFileExtRe.MatchString(text) {
				b.FileName = text
			}
		}
		current = current[:0]
	}
	for i := 0; i <= len(b.Raw); i++ {
		if i < len(b.Raw) && b.Raw[i] >= 32 && b.Raw[i] <= 126 {
			if len(current) == 0 {
				start = i
			}
			current = append(current, b.Raw[i])
			continue
		}
		flush()
	}
}

// ParseApoBlob valida o framing mínimo (magic 5 bytes + kind) do blob APO.
// O restante do layout (tabelas de records) ainda não é100% decifrado —
// ver docs/RPO-EXTRACTION-METHODOLOGY.md; as tabelas derivadas (strings,
// literais) são preenchidas por varredura nas tasks seguintes.
func ParseApoBlob(data []byte) (*ApoBlob, error) {
	if len(data) < len(ApoHeaderPrefix)+1 {
		return nil, fmt.Errorf("apo: blob muito pequeno (%d bytes)", len(data))
	}
	for i, b := range ApoHeaderPrefix {
		if data[i] != b {
			return nil, fmt.Errorf("apo: magic inválida no offset %d (0x%02X != 0x%02X)", i, data[i], b)
		}
	}
	kind := ApoKind(data[5])
	if kind != ApoKindAdvPL && kind != ApoKindTLPP {
		return nil, fmt.Errorf("apo: kind desconhecido 0x%02X (esperado 'F' ou 'T')", data[5])
	}
	return &ApoBlob{Raw: data, Kind: kind, Size: len(data)}, nil
}
