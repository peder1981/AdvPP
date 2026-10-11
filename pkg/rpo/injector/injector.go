// Package injector fornece funcionalidades para injetar bytecode em RPOs
// do Protheus, decodificando, modificando e recodificando o conteúdo.
package injector

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"unsafe"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/advpl/compiler/pkg/rpo"
)

type APORecord struct {
	Offset     int
	Size       int
	Name       string
	Timestamp  float64
	BuildType  int
	BinaryType int
	Code       []byte
}

type Injector struct {
	rawData      []byte
	adminSection []byte
	body         []byte
	selfOffset   uint32
	segments     []rpo.CaptureSegment
	decoded      map[int][]byte
	modified     map[int][]byte
	apobodies    map[int][]byte
	segMap       map[int]*rpo.CaptureSegment
	simpleMode   bool
	adminIndex   *AdminIndex
}

func NewInjector(data []byte) (*Injector, error) {
	if len(data) < 72 {
		return nil, fmt.Errorf("RPO muito pequeno (%d bytes)", len(data))
	}
	selfOffset := binary.LittleEndian.Uint32(data[0:4])
	footerStart := len(data) - 34
	if footerStart <= 38 || selfOffset > uint32(footerStart) {
		return nil, fmt.Errorf("RPO inválido: selfOffset=%d fora dos limites", selfOffset)
	}
	return &Injector{
		rawData:      data,
		adminSection: data[38:selfOffset],
		body:         data[selfOffset:footerStart],
		selfOffset:   selfOffset,
		decoded:      make(map[int][]byte),
		modified:     make(map[int][]byte),
		apobodies:    make(map[int][]byte),
		segMap:       make(map[int]*rpo.CaptureSegment),
	}, nil
}

func (inj *Injector) RPOName() string {
	if len(inj.rawData) < 16 {
		return ""
	}
	return string(bytes.TrimRight(inj.rawData[4:16], "\x00"))
}

func (inj *Injector) Body() []byte { return inj.body }
func (inj *Injector) SelfOffset() uint32 { return inj.selfOffset }

func (inj *Injector) LoadCapture(path string) error {
	events, err := rpo.LoadCaptureEvents(path)
	if err != nil {
		return fmt.Errorf("lendo captura: %w", err)
	}
	inj.segments = rpo.MergeCaptureSegments(events)
	fmt.Printf("Carregados %d segmentos de captura\n", len(inj.segments))
	return nil
}

func (inj *Injector) DecodeBody() ([]byte, map[int][]byte) {
	for _, seg := range inj.segments {
		if seg.Cipher == "" || seg.Key == "" || seg.Plaintext == "" {
			continue
		}
		key, _ := hex.DecodeString(seg.Key)
		var iv []byte
		if seg.IV != "" {
			iv, _ = hex.DecodeString(seg.IV)
		}
		plain, _ := hex.DecodeString(seg.Plaintext)
		ct, err := rpo.EncryptSegment(seg.Cipher, key, iv, plain)
		if err != nil {
			fmt.Printf("  Segmento #%d (%s): encrypt falhou: %v\n", seg.N, seg.Cipher, err)
			continue
		}
		idx := bytes.Index(inj.body, ct)
		if idx < 0 {
			idx = bytes.Index(inj.adminSection, ct)
			if idx < 0 {
				fmt.Printf("  Segmento #%d (%s): NÃO encontrado\n", seg.N, seg.Cipher)
				continue
			}
			fmt.Printf("  Segmento #%d (%s): ADMIN offset %d (%d bytes)\n", seg.N, seg.Cipher, idx, len(plain))
		} else {
			fmt.Printf("  Segmento #%d (%s): BODY offset %d (%d bytes)\n", seg.N, seg.Cipher, idx, len(plain))
		}
		inj.decoded[idx] = plain
		inj.segMap[idx] = &seg
		if len(plain) >= 2 && plain[0] == 0x78 {
			if reader, err := zlib.NewReader(bytes.NewReader(plain)); err == nil {
				if inflated, err := io.ReadAll(reader); err == nil {
					fmt.Printf("    zlib inflate OK (%d bytes)\n", len(inflated))
					inj.apobodies[idx] = inflated
				}
				reader.Close()
			}
		}
	}
	return inj.body, inj.decoded
}

func (inj *Injector) GetAPORecords() []*APORecord {
	var allRecords []*APORecord
	// Ordenar chaves para garantir ordem determinística
	keys := make([]int, 0, len(inj.apobodies))
	for k := range inj.apobodies {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	// Combinar segmentos na ordem dos offsets
	combined := make([]byte, 0)
	for _, k := range keys {
		combined = append(combined, inj.apobodies[k]...)
	}
	if len(combined) > 0 {
		allRecords = append(allRecords, extractAPORecords(combined)...)
	}
	return allRecords
}

// ReplaceFunction substitui o bytecode de uma função pelo nome.
func (inj *Injector) ReplaceFunction(name string, newCode []byte) error {
	records := inj.GetAPORecords()
	for _, rec := range records {
		if rec.Name == name {
			fmt.Printf("Substituindo APO #%s: %s\n", name, rec.Name)
			fmt.Printf("  Código: %d -> %d bytes\n", len(rec.Code), len(newCode))
			return inj.replaceInEncryptedMode(rec, newCode)
		}
	}
	return fmt.Errorf("função '%s' não encontrada", name)
}

// ReplaceFunctions substitui bytecodes de múltiplas funções (batch).
func (inj *Injector) ReplaceFunctions(pairs map[string][]byte) error {
	if len(pairs) == 0 {
		return nil
	}
	fmt.Printf("\n=== Injeção em lote: %d funções ===\n", len(pairs))
	success := 0
	for name, code := range pairs {
		if err := inj.ReplaceFunction(name, code); err != nil {
			fmt.Printf("  ✗ %s: %v\n", name, err)
		} else {
			success++
		}
	}
	fmt.Printf("Resultado: %d/%d funções injetadas com sucesso\n", success, len(pairs))
	if success == 0 {
		return fmt.Errorf("nenhuma função encontrada para injeção")
	}
	return nil
}

// SavePadded salva o RPO mantendo o tamanho total original.
func (inj *Injector) Save(path string) error {
	modifiedBody, err := inj.rebuildBody()
	if err != nil {
		return err
	}
	newData := inj.rebuildRPO(modifiedBody)
	return os.WriteFile(path, newData, 0644)
}

func (inj *Injector) SavePadded(path string) error {
	modifiedBody, err := inj.rebuildBody()
	if err != nil {
		return err
	}
	newData := inj.rebuildRPO(modifiedBody)
	return os.WriteFile(path, newData, 0644)
}

// DetectSimpleMode verifica se o corpo é um formato APO plano (não criptografado).
func (inj *Injector) DetectSimpleMode() {
	if len(inj.body) < 20 {
		return
	}
	codeSize := int(binary.LittleEndian.Uint32(inj.body[0:4]))
	if codeSize > 0 && codeSize < 10*1024*1024 {
		nameEnd := bytes.IndexByte(inj.body[4:], 0)
		if nameEnd >= 0 && nameEnd < 200 {
			name := string(inj.body[4 : 4+nameEnd])
			if len(name) >= 2 {
				afterName := 4 + nameEnd + 1
				if afterName+16 <= len(inj.body) {
					ts := uint64ToFloat(binary.LittleEndian.Uint64(inj.body[afterName:afterName+8]))
					if ts > 40000 && ts < 50000 {
						inj.simpleMode = true
						inj.apobodies[0] = inj.body
						inj.decoded[0] = inj.body
						fmt.Printf("  Modo simples detectado: %d bytes APO plano\n", len(inj.body))
						return
					}
				}
			}
		}
	}
}

// ParseAdminIndex tenta parsear o índice APO na admin section.
func (inj *Injector) ParseAdminIndex() error {
	idx, n, err := ParseAdminIndex(inj.adminSection)
	if err == nil && len(idx.Entries) > 0 {
		inj.adminIndex = idx
		fmt.Printf("  Índice APO parseado: %d entries (%d bytes consumidos)\n", len(idx.Entries), n)
		return nil
	}
	for start := 0; start < min(100, len(inj.adminSection)-8); start++ {
		idx, n, err = ParseAdminIndex(inj.adminSection[start:])
		if err == nil && len(idx.Entries) > 0 {
			inj.adminIndex = idx
			fmt.Printf("  Índice APO encontrado em offset %d: %d entries\n", start, len(idx.Entries))
			return nil
		}
	}
	fmt.Printf("  Admin section não parseável como índice APO (provavelmente criptografado)\n")
	return nil
}

// AdminIndex retorna o índice admin parseado.
func (inj *Injector) AdminIndex() *AdminIndex {
	return inj.adminIndex
}

func (inj *Injector) rebuildBody() ([]byte, error) {
	newBody := make([]byte, len(inj.body))
	copy(newBody, inj.body)

	for off, origPlain := range inj.decoded {
		modified, hasMod := inj.modified[off]
		if !hasMod {
			continue
		}
		seg, hasSeg := inj.segMap[off]
		if !hasSeg {
			continue
		}
		padded := make([]byte, len(origPlain))
		if len(modified) > len(padded) {
			copy(padded, modified[:len(origPlain)])
		} else {
			copy(padded, modified)
			for i := len(modified); i < len(padded); i++ {
				padded[i] = origPlain[i]
			}
		}
		var buf bytes.Buffer
		zw := zlib.NewWriter(&buf)
		zw.Write(padded)
		zw.Close()
		compressed := buf.Bytes()
		key, _ := hex.DecodeString(seg.Key)
		iv, _ := hex.DecodeString(seg.IV)
		ct, err := rpo.EncryptSegment(seg.Cipher, key, iv, compressed)
		if err != nil {
			fmt.Printf("  Erro ao recriptografar #%d (%s): %v\n", seg.N, seg.Cipher, err)
			continue
		}
		if len(ct) > len(origPlain) {
			ct = ct[:len(origPlain)]
		} else if len(ct) < len(origPlain) {
			ct = append(ct, make([]byte, len(origPlain)-len(ct))...)
		}
		copy(newBody[off:off+len(ct)], ct)
		fmt.Printf("  Segmento #%d (%s): substituído (%d bytes)\n", seg.N, seg.Cipher, len(ct))
	}
	return newBody, nil
}

func (inj *Injector) rebuildRPO(modifiedBody []byte) []byte {
	header := inj.rawData[0:38]
	footerMagic := inj.rawData[len(inj.rawData)-34 : len(inj.rawData)-24]
	footerTrail := inj.rawData[len(inj.rawData)-24:]
	f := &rpo.File{
		Header:       header,
		AdminSection: inj.adminSection,
		Body:         modifiedBody,
		FooterMagic:  string(footerMagic),
		FooterTrail:  footerTrail,
	}
	newData := f.Bytes()
	binary.LittleEndian.PutUint32(newData[0:4], uint32(len(header)+len(inj.adminSection)))
	return newData
}

func (inj *Injector) replaceInEncryptedMode(rec *APORecord, newCode []byte) error {
	for off, inflated := range inj.apobodies {
		records := extractAPORecords(inflated)
		for _, r := range records {
			if r.Name == rec.Name {
				oldSize := len(r.Code)
				padded := make([]byte, oldSize)
				copy(padded, newCode)
				if len(newCode) > oldSize {
					padded = newCode[:oldSize]
				}
				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, uint32(len(padded)))
				buf.WriteString(r.Name)
				buf.WriteByte(0)
				binary.Write(&buf, binary.LittleEndian, floatToUint64(r.Timestamp))
				binary.Write(&buf, binary.LittleEndian, uint32(r.BuildType))
				binary.Write(&buf, binary.LittleEndian, uint32(r.BinaryType))
				buf.Write(padded)
				newInflated := bytes.Replace(inflated, r.Code, padded, 1)
				inj.apobodies[off] = newInflated
				inj.modified[off] = newInflated
				fmt.Printf("  Função '%s': %d -> %d bytes (pad/trunc)\n", r.Name, oldSize, len(padded))
				return nil
			}
		}
	}
	return fmt.Errorf("registro APO '%s' não encontrado no body criptografado", rec.Name)
}

// ExtractAPORecords extrai registros APO de dados decomprimidos.
func ExtractAPORecords(data []byte) []*APORecord {
	return extractAPORecords(data)
}

func extractAPORecords(data []byte) []*APORecord {
	var records []*APORecord
	offset := 0
	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}
		codeSize := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		recordStart := offset
		offset += 4
		if codeSize == 0 {
			offset++
			continue
		}
		if codeSize > 10*1024*1024 {
			break
		}
		nullIdx := bytes.IndexByte(data[offset:], 0)
		if nullIdx < 0 || nullIdx > 200 {
			break
		}
		name := string(data[offset : offset+nullIdx])
		nameWithNull := nullIdx + 1
		offset += nameWithNull
		if len(name) < 2 {
			continue
		}
		if offset+16 > len(data) {
			break
		}
		tsBits := binary.LittleEndian.Uint64(data[offset : offset+8])
		timestamp := uint64ToFloat(tsBits)
		buildType := int(binary.LittleEndian.Uint32(data[offset+8 : offset+12]))
		binaryType := int(binary.LittleEndian.Uint32(data[offset+12 : offset+16]))
		offset += 16
		if offset+codeSize > len(data) {
			break
		}
		code := make([]byte, codeSize)
		copy(code, data[offset:offset+codeSize])
		offset += codeSize
		records = append(records, &APORecord{
			Offset: recordStart, Size: offset - recordStart, Name: name,
			Timestamp: timestamp, BuildType: buildType, BinaryType: binaryType, Code: code,
		})
	}
	return records
}

func PrintAPORecords(records []*APORecord) {
	fmt.Printf("Encontrados %d registros APO:\n", len(records))
	for i, rec := range records {
		fmt.Printf("\n%d. %s (offset=%d, size=%d, code=%d bytes)\n",
			i+1, rec.Name, rec.Offset, rec.Size, len(rec.Code))
	}
}

func PrintAdminIndex(idx *AdminIndex) {
	if idx == nil {
		fmt.Println("Nenhum índice admin parseado")
		return
	}
	fmt.Printf("Índice APO: %d entries\n", len(idx.Entries))
	for i, e := range idx.Entries {
		fmt.Printf("  %d. %s (ts=%.2f, f4c=%d, f81=%d)\n", i+1, e.Name, e.Timestamp, e.Field4C, e.Field81)
	}
}

func SortRecords(records []*APORecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].Offset < records[j].Offset
	})
}

func floatToUint64(f float64) uint64 {
	return *(*uint64)(unsafe.Pointer(&f))
}

func uint64ToFloat(u uint64) float64 {
	return *(*float64)(unsafe.Pointer(&u))
}

func ParseAPOIndex(data []byte) ([]*AdminIndexEntry, int, error) {
	idx, n, err := ParseAdminIndex(data)
	if err != nil {
		return nil, 0, err
	}
	entries := make([]*AdminIndexEntry, len(idx.Entries))
	for i, e := range idx.Entries {
		entries[i] = &AdminIndexEntry{
			Name: e.Name, Timestamp: e.Timestamp,
			Field4C: e.Field4C, Field50: e.Field50, Field44: e.Field44,
			Field48: e.Field48, Field20: e.Field20, Field24: e.Field24,
			Field40: e.Field40, Field60: e.Field60, Field81: e.Field81, HasFull: true,
		}
	}
	return entries, n, nil
}

func SerializeAPOIndex(entries []*AdminIndexEntry) []byte {
	idx := &AdminIndex{}
	for _, e := range entries {
		idx.Entries = append(idx.Entries, &AdminIndexEntry{
			Name: e.Name, Timestamp: e.Timestamp,
			Field4C: e.Field4C, Field50: e.Field50, Field44: e.Field44,
			Field48: e.Field48, Field20: e.Field20, Field24: e.Field24,
			Field40: e.Field40, Field60: e.Field60, Field81: e.Field81, HasFull: true,
		})
	}
	return SerializeAdminIndex(idx)
}

func readZlib(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
