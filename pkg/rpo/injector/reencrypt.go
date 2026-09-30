package injector

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// ReEncryptionManager gerencia a recriptografia de segmentos do RPO.
type ReEncryptionManager struct {
	segments []CaptureSegment
	body     []byte
	admin    []byte
}

// NewReEncryptionManager cria um gerenciador de recriptografia.
func NewReEncryptionManager(body, admin []byte) *ReEncryptionManager {
	return &ReEncryptionManager{
		body:  body,
		admin: admin,
	}
}

// AddSegment adiciona um segmento de captura para recriptografia.
func (rem *ReEncryptionManager) AddSegment(seg CaptureSegment) {
	rem.segments = append(rem.segments, seg)
}

// SortSegments ordena segmentos por número para processamento correto.
func (rem *ReEncryptionManager) SortSegments() {
	sort.Slice(rem.segments, func(i, j int) bool {
		return rem.segments[i].N < rem.segments[j].N
	})
}

// RecompressBytes comprime dados com zlib.
func RecompressBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.DefaultCompression)
	if err != nil {
		return nil, fmt.Errorf("criando compressor zlib: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		w.Close()
		return nil, fmt.Errorf("escrevendo dados no zlib: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("fechando compressor zlib: %w", err)
	}
	return buf.Bytes(), nil
}

// DecompressBytes decomprime dados zlib.
func DecompressBytes(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("criando decompressor zlib: %w", err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

// FindSegmentInBody procura um segmento no body pelo plaintext.
func (rem *ReEncryptionManager) FindSegmentInBody(plaintext []byte) int {
	return bytes.Index(rem.body, plaintext)
}

// FindSegmentInAdmin procura um segmento no admin section.
func (rem *ReEncryptionManager) FindSegmentInAdmin(plaintext []byte) int {
	return bytes.Index(rem.admin, plaintext)
}

// ProcessAllSegments processa todos os segmentos e retorna mapeamento offset -> plaintext.
func (rem *ReEncryptionManager) ProcessAllSegments() (map[int][]byte, error) {
	results := make(map[int][]byte)
	
	for i := range rem.segments {
		seg := &rem.segments[i]
		
		plain, err := hex.DecodeString(seg.Plaintext)
		if err != nil {
			fmt.Printf("  Segmento #%d: erro no plaintext: %v\n", seg.N, err)
			continue
		}
		
		// Verificar se é zlib
		var targetPlain []byte
		if len(plain) >= 2 && plain[0] == 0x78 {
			if inflated, err := DecompressBytes(plain); err == nil {
				targetPlain = inflated
			} else {
				targetPlain = plain
			}
		} else {
			targetPlain = plain
		}
		
		// Encontrar no body ou admin
		offset := rem.FindSegmentInBody(targetPlain)
		if offset < 0 {
			offset = rem.FindSegmentInAdmin(targetPlain)
		}
		
		if offset >= 0 {
			results[offset] = targetPlain
			fmt.Printf("  Segmento #%d (%s): encontrado em offset %d, tamanho %d\n", 
				seg.N, seg.Cipher, offset, len(targetPlain))
		} else {
			fmt.Printf("  Segmento #%d (%s): NÃO encontrado\n", seg.N, seg.Cipher)
		}
	}
	
	return results, nil
}

// ParseCipherInfo extrai informações de cipher do nome.
func ParseCipherInfo(cipherName string) (string, string, error) {
	base := cipherName
	mode := "unknown"
	
	switch {
	case bytes.HasSuffix([]byte(cipherName), []byte("_ecb_cipher")):
		base = cipherName[:len(cipherName)-len("_ecb_cipher")]
		mode = "ECB"
	case bytes.HasSuffix([]byte(cipherName), []byte("_cbc_cipher")):
		base = cipherName[:len(cipherName)-len("_cbc_cipher")]
		mode = "CBC"
	case bytes.HasSuffix([]byte(cipherName), []byte("_cfb64_cipher")):
		base = cipherName[:len(cipherName)-len("_cfb64_cipher")]
		mode = "CFB64"
	case bytes.HasSuffix([]byte(cipherName), []byte("_ofb_cipher")):
		base = cipherName[:len(cipherName)-len("_ofb_cipher")]
		mode = "OFB"
	}
	
	return base, mode, nil
}

// GenerateCaptureJSON gera o JSON de captura a partir de segmentos processados.
func GenerateCaptureJSON(segments []CaptureSegment) ([]byte, error) {
	var events []map[string]interface{}
	
	for _, seg := range segments {
		events = append(events, map[string]interface{}{
			"N":         seg.N,
			"Type":      "evpinit",
			"Cipher":    seg.Cipher,
			"Key":       seg.Key,
			"IV":        seg.IV,
			"Plaintext": "",
		})
	}
	
	return json.MarshalIndent(events, "", "  ")
}
