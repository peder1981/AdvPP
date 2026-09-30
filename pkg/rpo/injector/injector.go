// Package injector fornece funcionalidades para injetar bytecode em RPOs
// do Protheus, decodificando, modificando e recodificando o conteúdo.
package injector

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"unsafe"

	"github.com/advpl/compiler/pkg/rpo"
)

// APORecord representa um registro APO no RPO.
type APORecord struct {
	Offset     int
	Size       int
	Name       string
	Timestamp  float64
	BuildType  int
	BinaryType int
	Code       []byte
}

// Segmento de captura.
type CaptureSegment struct {
	N         int
	Cipher    string
	Key       string
	IV        string
	Plaintext string
}

// Injector manipula o conteúdo de um RPO.
type Injector struct {
	rawData      []byte
	adminSection []byte
	body         []byte
	selfOffset   uint32
	segments     []rpo.CaptureSegment
	decoded      map[int][]byte      // offset no body -> plaintext
	modified     map[int][]byte      // offset no body -> novo plaintext
	apobodies    map[int][]byte      // offset no body -> decompressed
	segMap       map[int]*rpo.CaptureSegment // offset -> segmento
}

// NewInjector cria um novo injector.
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

// RPOName retorna o nome do RPO.
func (inj *Injector) RPOName() string {
	if len(inj.rawData) < 16 {
		return ""
	}
	return string(bytes.TrimRight(inj.rawData[4:16], "\x00"))
}

// Body retorna o body.
func (inj *Injector) Body() []byte {
	return inj.body
}

// SelfOffset retorna o self offset.
func (inj *Injector) SelfOffset() uint32 {
	return inj.selfOffset
}

// LoadCapture carrega segmentos de captura.
func (inj *Injector) LoadCapture(path string) error {
	events, err := rpo.LoadCaptureEvents(path)
	if err != nil {
		return fmt.Errorf("lendo captura: %w", err)
	}
	inj.segments = rpo.MergeCaptureSegments(events)
	fmt.Printf("Carregados %d segmentos de captura\n", len(inj.segments))
	return nil
}

// DecodeBody decodifica o body.
func (inj *Injector) DecodeBody() ([]byte, map[int][]byte) {
	for _, seg := range inj.segments {
		if seg.Cipher == "" || seg.Key == "" || seg.Plaintext == "" {
			continue
		}

		key, err := hex.DecodeString(seg.Key)
		if err != nil {
			fmt.Printf("  Segmento #%d %s: chave inválida\n", seg.N, seg.Cipher)
			continue
		}

		var iv []byte
		if seg.IV != "" {
			iv, _ = hex.DecodeString(seg.IV)
		}

		plain, err := hex.DecodeString(seg.Plaintext)
		if err != nil {
			fmt.Printf("  Segmento #%d %s: plaintext inválido\n", seg.N, seg.Cipher)
			continue
		}

		ct, err := rpo.EncryptSegment(seg.Cipher, key, iv, plain)
		if err != nil {
			fmt.Printf("  Segmento #%d (%s): encrypt falhou: %v\n", seg.N, seg.Cipher, err)
			continue
		}

		idx := bytes.Index(inj.body, ct)
		if idx >= 0 {
			inj.decoded[idx] = plain
			inj.segMap[idx] = &seg
			fmt.Printf("  Segmento #%d (%s): BODY offset %d (%d bytes)\n", seg.N, seg.Cipher, idx, len(plain))
			
			if len(plain) >= 2 && plain[0] == 0x78 {
				if r, err := zlib.NewReader(bytes.NewReader(plain)); err == nil {
					if inflated, err := io.ReadAll(r); err == nil {
						fmt.Printf("    zlib inflate OK (%d bytes)\n", len(inflated))
						inj.apobodies[idx] = inflated
					}
					r.Close()
				}
			}
		} else {
			idx = bytes.Index(inj.adminSection, ct)
			if idx >= 0 {
				inj.decoded[idx] = plain
				inj.segMap[idx] = &seg
				fmt.Printf("  Segmento #%d (%s): ADMIN offset %d (%d bytes)\n", seg.N, seg.Cipher, idx, len(plain))
			} else {
				fmt.Printf("  Segmento #%d (%s): NÃO encontrado\n", seg.N, seg.Cipher)
			}
		}
	}

	return inj.body, inj.decoded
}

// GetAPORecords extrai registros APO.
func (inj *Injector) GetAPORecords() []*APORecord {
	var allRecords []*APORecord
	
	combined := make([]byte, 0)
	for _, b := range inj.apobodies {
		combined = append(combined, b...)
	}
	
	if len(combined) > 0 {
		allRecords = extractAPORecords(combined)
	}
	
	return allRecords
}

// ReplaceAPO substitui o código de um registro APO.
func (inj *Injector) ReplaceAPO(recordIdx int, newCode []byte) error {
	records := inj.GetAPORecords()
	
	if recordIdx < 0 || recordIdx >= len(records) {
		return fmt.Errorf("índice inválido: %d (total: %d)", recordIdx, len(records))
	}

	rec := records[recordIdx]
	fmt.Printf("Substituindo APO #%d: %s\n", recordIdx+1, rec.Name)
	fmt.Printf("  Código: %d -> %d bytes\n", len(rec.Code), len(newCode))

	for off, body := range inj.apobodies {
		newBody := modifyAPOBody(body, records, recordIdx, newCode)
		if newBody != nil {
			inj.apobodies[off] = newBody
			inj.modified[off] = newBody
			fmt.Printf("  Body atualizado em offset %d\n", off)
		}
	}
	
	return nil
}

// InjectBytecode injeta bytecode de arquivo.
func (inj *Injector) InjectBytecode(recordIdx int, bytecodePath string) error {
	code, err := os.ReadFile(bytecodePath)
	if err != nil {
		return fmt.Errorf("lendo bytecode: %w", err)
	}
	return inj.ReplaceAPO(recordIdx, code)
}

// Save salva o RPO modificado.
func (inj *Injector) Save(path string) error {
	newBody, err := inj.rebuildBody()
	if err != nil {
		return err
	}

	newData := make([]byte, len(inj.rawData))
	copy(newData, inj.rawData)
	copy(newData[inj.selfOffset:], newBody)

	return os.WriteFile(path, newData, 0644)
}

// rebuildBody reconstrói o body com recriptografia.
func (inj *Injector) rebuildBody() ([]byte, error) {
	newBody := make([]byte, len(inj.body))
	copy(newBody, inj.body)
	
	for off, modified := range inj.modified {
		seg, hasSeg := inj.segMap[off]
		if !hasSeg {
			continue
		}
		
		origPlain, hasPlain := inj.decoded[off]
		if !hasPlain {
			continue
		}
		
		// Recomprimir
		var buf bytes.Buffer
		zw := zlib.NewWriter(&buf)
		zw.Write(modified)
		zw.Close()
		compressed := buf.Bytes()
		
		// Recriptografar
		key, _ := hex.DecodeString(seg.Key)
		iv, _ := hex.DecodeString(seg.IV)
		ct, err := rpo.EncryptSegment(seg.Cipher, key, iv, compressed)
		if err != nil {
			fmt.Printf("  Erro ao recriptografar #%d: %v\n", seg.N, err)
			continue
		}
		
		// Substituir no body usando o offset conhecido
		sizeDiff := len(ct) - len(origPlain)
		if sizeDiff == 0 {
			copy(newBody[off:off+len(ct)], ct)
		} else if sizeDiff > 0 {
			// Cresceu - realocar
			oldEnd := off + len(origPlain)
			expanded := make([]byte, len(newBody)+sizeDiff)
			copy(expanded, newBody[:off])
			copy(expanded[off:off+len(ct)], ct)
			copy(expanded[off+len(ct):], newBody[oldEnd:])
			newBody = expanded
		} else {
			// Encolheu - realocar
			oldEnd := off + len(origPlain)
			contracted := make([]byte, len(newBody)+sizeDiff)
			copy(contracted, newBody[:off])
			copy(contracted[off:off+len(ct)], ct)
			copy(contracted[off+len(ct):], newBody[oldEnd:])
			newBody = contracted
		}
		
		fmt.Printf("  Segmento #%d: substituído (%d -> %d bytes)\n", seg.N, len(origPlain), len(ct))
	}
	
	inj.body = newBody
	return newBody, nil
}

// modifyAPOBody modifica um body APO com novo código.
func modifyAPOBody(original []byte, records []*APORecord, recordIdx int, newCode []byte) []byte {
	if recordIdx < 0 || recordIdx >= len(records) {
		return nil
	}
	
	var buf bytes.Buffer
	offset := 0
	
	for i, rec := range records {
		if rec.Offset > offset {
			buf.Write(original[offset:rec.Offset])
		}
		
		code := rec.Code
		if i == recordIdx {
			code = newCode
		}
		
		binary.Write(&buf, binary.LittleEndian, uint32(len(code)))
		buf.WriteString(rec.Name)
		buf.WriteByte(0)
		
		tsBits := floatToUint64(rec.Timestamp)
		binary.Write(&buf, binary.LittleEndian, tsBits)
		
		binary.Write(&buf, binary.LittleEndian, uint32(rec.BuildType))
		binary.Write(&buf, binary.LittleEndian, uint32(rec.BinaryType))
		
		buf.Write(code)
		
		offset = rec.Offset + rec.Size
	}
	
	if len(original) > offset {
		buf.Write(original[offset:])
	}
	
	return buf.Bytes()
}

// ExtractAPORecords extrai registros APO.
func ExtractAPORecords(data []byte) []*APORecord {
	return extractAPORecords(data)
}

// extractAPORecords extrai registros APO.
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

		if codeSize == 0 || codeSize > 10*1024*1024 {
			offset++
			continue
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

		if offset+8 > len(data) {
			break
		}
		tsBits := binary.LittleEndian.Uint64(data[offset : offset+8])
		timestamp := uint64ToFloat(tsBits)
		offset += 8

		if offset+4 > len(data) {
			break
		}
		buildType := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+4 > len(data) {
			break
		}
		binaryType := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4

		if offset+codeSize > len(data) {
			break
		}
		code := make([]byte, codeSize)
		copy(code, data[offset:offset+codeSize])
		offset += codeSize

		totalSize := offset - recordStart
		records = append(records, &APORecord{
			Offset:     recordStart,
			Size:       totalSize,
			Name:       name,
			Timestamp:  timestamp,
			BuildType:  buildType,
			BinaryType: binaryType,
			Code:       code,
		})
	}

	return records
}

// PrintAPORecords imprime registros APO.
func PrintAPORecords(records []*APORecord) {
	fmt.Printf("Encontrados %d registros APO:\n", len(records))
	for i, rec := range records {
		fmt.Printf("\n%d. %s (offset=%d, size=%d, code=%d bytes)\n", 
			i+1, rec.Name, rec.Offset, rec.Size, len(rec.Code))
	}
}

// SortRecords ordena por offset.
func SortRecords(records []*APORecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].Offset < records[j].Offset
	})
}

// floatToUint64 converte float64 para uint64.
func floatToUint64(f float64) uint64 {
	return *(*uint64)(unsafe.Pointer(&f))
}

// uint64ToFloat converte uint64 para float64.
func uint64ToFloat(u uint64) float64 {
	return *(*float64)(unsafe.Pointer(&u))
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
