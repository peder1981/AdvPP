package rpo

// apo_parser.go — scanner EXPERIMENTAL e CONSERVADOR de candidatos a
// registros APO (Advanced Program Object) dentro do conteúdo do RPO.
//
// [!] AVISO DE HONESTIDADE (leia antes de usar):
//
// Os registros APO REAIS vivem no conteúdo DECIFRADO e DESCOMPRIMIDO do
// RPO (zlib). O conteúdo no disco é cifra alta-entropia. Um scanner que
// procura "size+type" diretamente no conteúdo cifrado casa com RUÍDO
// estatístico — a taxa de falso positivo é indistinguível da de dados
// aleatórios do mesmo tamanho. Ver pkg/rpo/forensics.go e
// forensics_test.go (prova empírica) e docs/RPO-GROUND-TRUTH.md.
//
// Portanto:
//   - Este scanner NÃO é a forma correta de ler APOs. A forma correta é
//     `advplc rpo decrypt` (com captura ao vivo) e então parsear o
//     plaintext zlib.
//   - Ele existe apenas como utilitário de inspeção para conteúdo JÁ
//     decifrado (plaintext) ou para triagem, e é deliberadamente estrito:
//     parte de confiança 0.0 e só eleva com EVIDÊNCIA verificável.
//   - Em conteúdo cifrado, o resultado esperado é ~nenhum candidato
//     acima de DefaultAPOMinConfidence. Se muitos aparecerem, é bug/ruído.
//
// Os nomes de tipo (APO_TYPE_*) abaixo foram inferidos de análise do
// produto e NÃO são uma tabela oficial confirmada da TOTVS.

import (
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
)

// Tipos de registro APO (inferidos, não confirmados oficialmente).
const (
	APO_TYPE_FUNCTION   = 0x01
	APO_TYPE_METHOD     = 0x02
	APO_TYPE_CLASS      = 0x03
	APO_TYPE_PROPERTY   = 0x04
	APO_TYPE_VARIABLE   = 0x05
	APO_TYPE_CONSTANT   = 0x06
	APO_TYPE_INCLUDE    = 0x07
	APO_TYPE_LIBRARY    = 0x08
	APO_TYPE_NAMESPACE  = 0x09
	APO_TYPE_EVENT      = 0x0A
	APO_TYPE_TRIGGER    = 0x0B
	APO_TYPE_INDEX      = 0x0C
	APO_TYPE_RELATION   = 0x0D
	APO_TYPE_VALIDATION = 0x0E
	APO_TYPE_UI         = 0x0F
	APO_TYPE_REPORT     = 0x10
	APO_TYPE_MENU       = 0x11
	APO_TYPE_PROCESS    = 0x12
	APO_TYPE_SERVICE    = 0x13
	APO_TYPE_JOB        = 0x14
)

// DefaultAPOMinConfidence é o limiar mínimo recomendado. Alto de
// propósito: em conteúdo cifrado, nada deve passar deste limiar.
const DefaultAPOMinConfidence = 0.80

// APORecord é um candidato que passou o limiar.
type APORecord struct {
	Offset     int               `json:"offset"`
	Size       int               `json:"size"`
	Type       uint8             `json:"type"`
	TypeName   string            `json:"type_name"`
	Name       string            `json:"name"`
	Evidence   []string          `json:"evidence,omitempty"`
	Data       []byte            `json:"-"`
	Properties map[string]string `json:"properties,omitempty"`
}

// APOCandidate é um possível cabeçalho de registro APO.
type APOCandidate struct {
	Offset     int
	Size       uint32
	Type       uint8
	Confidence float64
	Evidence   []string
}

// APOParser varre um buffer em busca de candidatos a registros APO.
type APOParser struct {
	data       []byte
	records    []*APORecord
	candidates []APOCandidate
}

// NewAPOParser cria o parser para o buffer informado (idealmente plaintext).
func NewAPOParser(data []byte) *APOParser {
	return &APOParser{
		data:       data,
		records:    make([]*APORecord, 0),
		candidates: make([]APOCandidate, 0),
	}
}

// GetTypeName devolve o nome legível de um tipo APO.
func GetTypeName(typeId uint8) string {
	switch typeId {
	case APO_TYPE_FUNCTION:
		return "Function"
	case APO_TYPE_METHOD:
		return "Method"
	case APO_TYPE_CLASS:
		return "Class"
	case APO_TYPE_PROPERTY:
		return "Property"
	case APO_TYPE_VARIABLE:
		return "Variable"
	case APO_TYPE_CONSTANT:
		return "Constant"
	case APO_TYPE_INCLUDE:
		return "Include"
	case APO_TYPE_LIBRARY:
		return "Library"
	case APO_TYPE_NAMESPACE:
		return "Namespace"
	case APO_TYPE_EVENT:
		return "Event"
	case APO_TYPE_TRIGGER:
		return "Trigger"
	case APO_TYPE_INDEX:
		return "Index"
	case APO_TYPE_RELATION:
		return "Relation"
	case APO_TYPE_VALIDATION:
		return "Validation"
	case APO_TYPE_UI:
		return "UI"
	case APO_TYPE_REPORT:
		return "Report"
	case APO_TYPE_MENU:
		return "Menu"
	case APO_TYPE_PROCESS:
		return "Process"
	case APO_TYPE_SERVICE:
		return "Service"
	case APO_TYPE_JOB:
		return "Job"
	default:
		return fmt.Sprintf("Unknown_0x%02X", typeId)
	}
}

// ScanForCandidates varre o buffer por possíveis cabeçalhos. Cada
// candidato recebe confiança ESTRITA (base 0.0) e a lista de evidências.
func (p *APOParser) ScanForCandidates() []APOCandidate {
	p.candidates = make([]APOCandidate, 0)

	for i := 0; i+8 <= len(p.data); i += 4 {
		size := binary.LittleEndian.Uint32(p.data[i : i+4])
		typeId := p.data[i+4]

		// Pré-filtros duros (sem eles, nem vale pontuar).
		if size < 8 || size > 10*1024*1024 {
			continue
		}
		if typeId > 0x20 {
			continue
		}

		conf, ev := p.scoreCandidate(i, size, typeId)
		if conf < 0.5 {
			continue
		}
		p.candidates = append(p.candidates, APOCandidate{
			Offset:     i,
			Size:       size,
			Type:       typeId,
			Confidence: conf,
			Evidence:   ev,
		})
	}
	return p.candidates
}

// scoreCandidate calcula confiança ESTRITA, partindo de 0.0 e só somando
// com evidência verificável. Sem identificador válido, rejeita (0.0).
func (p *APOParser) scoreCandidate(offset int, size uint32, typeId uint8) (float64, []string) {
	conf := 0.0
	var ev []string

	// Evidência 1 (+0.40): identificador ASCII válido, null-terminado,
	// logo após o cabeçalho de 5 bytes.
	name, nameEnd := p.readIdentifierAt(offset + 5)
	if name == "" {
		return 0, nil // sem nome não há registro APO utilizável
	}
	conf += 0.40
	ev = append(ev, "identificador válido: "+name)

	// Evidência 2 (+0.10): tamanho declarado dentro dos limites.
	if int(size)+offset <= len(p.data) {
		conf += 0.10
		ev = append(ev, "tamanho dentro dos limites")
	}

	// Evidência 3 (+0.20): tipo conhecido.
	if GetTypeName(typeId) != fmt.Sprintf("Unknown_0x%02X", typeId) {
		conf += 0.20
		ev = append(ev, "tipo conhecido: "+GetTypeName(typeId))
	}

	// Evidência 4 (+0.20): após o nome vem outro campo length-prefixed
	// ASCII plausível (encadeamento típico do diretório RPO).
	if nameEnd+4 <= len(p.data) {
		nextLen := int(binary.LittleEndian.Uint32(p.data[nameEnd : nameEnd+4]))
		if nextLen > 0 && nextLen < 256 {
			conf += 0.20
			ev = append(ev, "campo seguinte length-prefixed plausível")
		}
	}

	// Evidência 5 (+0.10): alinhamento a 4 bytes.
	if offset%4 == 0 {
		conf += 0.10
		ev = append(ev, "alinhado a 4 bytes")
	}

	if conf > 1.0 {
		conf = 1.0
	}
	return conf, ev
}

// readIdentifierAt lê um identificador ASCII null-terminado em `off`.
// Devolve (nome, offset_apos_null) ou ("", off) se inválido.
func (p *APOParser) readIdentifierAt(off int) (string, int) {
	if off < 0 || off >= len(p.data) {
		return "", off
	}
	end := off
	for end < len(p.data) && p.data[end] != 0 {
		end++
		if end-off > 64 {
			return "", off
		}
	}
	if end >= len(p.data) {
		return "", off
	}
	name := string(p.data[off:end])
	if !isValidAdvplName(name) {
		return "", off
	}
	return name, end + 1
}

// isValidAdvplName valida um identificador AdvPL/TLPP (nome puro, sem
// pontos). Aceita até 64 chars para nomes de módulo.
func isValidAdvplName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	re := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	return re.MatchString(name)
}

// ParseRecords devolve os candidatos acima de minConfidence. Use
// DefaultAPOMinConfidence (0.85) — limiar baixo produz ruído.
func (p *APOParser) ParseRecords(minConfidence float64) []*APORecord {
	p.records = make([]*APORecord, 0)
	for _, cand := range p.ScanForCandidates() {
		if cand.Confidence < minConfidence {
			continue
		}
		if rec := p.extractRecord(cand); rec != nil {
			p.records = append(p.records, rec)
		}
	}
	return p.records
}

func (p *APOParser) extractRecord(cand APOCandidate) *APORecord {
	end := cand.Offset + int(cand.Size)
	if end > len(p.data) {
		end = len(p.data)
	}
	data := p.data[cand.Offset:end]

	rec := &APORecord{
		Offset:     cand.Offset,
		Size:       int(cand.Size),
		Type:       cand.Type,
		TypeName:   GetTypeName(cand.Type),
		Name:       func() string { n, _ := p.readIdentifierAt(cand.Offset + 5); return n }(),
		Evidence:   cand.Evidence,
		Data:       data,
		Properties: map[string]string{},
	}
	if rec.Name != "" {
		rec.Properties["name"] = rec.Name
	}
	return rec
}

// ExtractFunctions devolve nomes de candidatos de tipo Function/Method.
// Em conteúdo cifrado o resultado esperado é vazio.
func (p *APOParser) ExtractFunctions() []string {
	out := make([]string, 0)
	for _, r := range p.ParseRecords(DefaultAPOMinConfidence) {
		if (r.Type == APO_TYPE_FUNCTION || r.Type == APO_TYPE_METHOD) && r.Name != "" {
			out = append(out, r.Name)
		}
	}
	return out
}

// ExtractAll devolve um resumo estruturado.
func (p *APOParser) ExtractAll() map[string]interface{} {
	records := p.ParseRecords(DefaultAPOMinConfidence)
	result := map[string]interface{}{
		"total_records": len(records),
		"by_type":       map[string]int{},
		"records":       []map[string]interface{}{},
		"note":          "scanner estrito; em conteúdo cifrado o esperado é 0 registros",
	}
	for _, r := range records {
		result["by_type"].(map[string]int)[r.TypeName]++
		result["records"] = append(result["records"].([]map[string]interface{}), map[string]interface{}{
			"offset":   r.Offset,
			"size":     r.Size,
			"type":     r.TypeName,
			"name":     r.Name,
			"evidence": r.Evidence,
		})
	}
	return result
}

// DumpToWriter escreve o relatório textual.
func (p *APOParser) DumpToWriter(w io.Writer) {
	records := p.ParseRecords(DefaultAPOMinConfidence)
	fmt.Fprintf(w, "=== APO (scanner estrito) ===\n")
	fmt.Fprintf(w, "Registros acima de %.2f: %d\n\n", DefaultAPOMinConfidence, len(records))
	for i, r := range records {
		fmt.Fprintf(w, "  [%d] +%d size=%d type=%s name=%s\n", i+1, r.Offset, r.Size, r.TypeName, r.Name)
		for _, e := range r.Evidence {
			fmt.Fprintf(w, "        - %s\n", e)
		}
	}
}

// FindCandidatesAboveThreshold devolve candidatos acima do limiar.
func (p *APOParser) FindCandidatesAboveThreshold(threshold float64) []APOCandidate {
	var out []APOCandidate
	for _, c := range p.ScanForCandidates() {
		if c.Confidence >= threshold {
			out = append(out, c)
		}
	}
	return out
}

// GetTopCandidates devolve os N candidatos de maior confiança.
func (p *APOParser) GetTopCandidates(n int) []APOCandidate {
	cs := p.ScanForCandidates()
	for i := 0; i < len(cs); i++ {
		for j := i + 1; j < len(cs); j++ {
			if cs[j].Confidence > cs[i].Confidence {
				cs[i], cs[j] = cs[j], cs[i]
			}
		}
	}
	if n > len(cs) {
		n = len(cs)
	}
	return cs[:n]
}
