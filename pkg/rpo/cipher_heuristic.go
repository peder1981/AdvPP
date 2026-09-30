package rpo

import (
	"encoding/hex"
	"fmt"
)

// CipherInfo contém informações sobre um cipher identificado
type CipherInfo struct {
	Name       string
	KeySize    int // -1 = variável (RC4)
	IVSize     int // 0 = sem IV
	BlockSize  int
	Mode       string // ECB, CBC, CFB, OFB, STREAM
	Confidence float64
}

// CipherHeuristic identifica o cipher baseado em key/IV sizes
type CipherHeuristic struct {
	knowCiphers []CipherInfo
}

// NewCipherHeuristic cria um novo heuristic identificador
func NewCipherHeuristic() *CipherHeuristic {
	h := &CipherHeuristic{}
	h.initKnowCiphers()
	return h
}

// initKnowCiphers inicializa a tabela de ciphers conhecidos
func (h *CipherHeuristic) initKnowCiphers() {
	h.knowCiphers = []CipherInfo{
		// DES (key=8, iv=0 ou 8)
		{Name: "des_ecb_cipher", KeySize: 8, IVSize: 0, BlockSize: 8, Mode: "ECB"},
		{Name: "des_cbc_cipher", KeySize: 8, IVSize: 8, BlockSize: 8, Mode: "CBC"},
		{Name: "des_cfb64_cipher", KeySize: 8, IVSize: 8, BlockSize: 8, Mode: "CFB64"},
		{Name: "des_ofb_cipher", KeySize: 8, IVSize: 8, BlockSize: 8, Mode: "OFB"},
		
		// 3DES/DES-EDE (key=24, iv=0 ou 8)
		{Name: "des_ede_ecb_cipher", KeySize: 24, IVSize: 0, BlockSize: 8, Mode: "ECB"},
		{Name: "des_ede_cbc_cipher", KeySize: 24, IVSize: 8, BlockSize: 8, Mode: "CBC"},
		{Name: "des_ede_cfb64_cipher", KeySize: 24, IVSize: 8, BlockSize: 8, Mode: "CFB64"},
		{Name: "des_ede_ofb_cipher", KeySize: 24, IVSize: 8, BlockSize: 8, Mode: "OFB"},
		
		// RC4 (key=5-40, iv=0)
		{Name: "rc4_cipher", KeySize: -1, IVSize: 0, BlockSize: 1, Mode: "STREAM"},
		{Name: "rc4_hmac_md5_cipher", KeySize: 16, IVSize: 0, BlockSize: 1, Mode: "STREAM"},
		
		// RC2 (key=16, iv=0 ou 8)
		{Name: "rc2_ecb_cipher", KeySize: 16, IVSize: 0, BlockSize: 8, Mode: "ECB"},
		{Name: "rc2_cbc_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "CBC"},
		{Name: "rc2_cfb64_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "CFB64"},
		{Name: "rc2_ofb_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "OFB"},
		
		// CAST5 (key=16, iv=0 ou 8)
		{Name: "cast5_ecb_cipher", KeySize: 16, IVSize: 0, BlockSize: 8, Mode: "ECB"},
		{Name: "cast5_cbc_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "CBC"},
		{Name: "cast5_cfb64_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "CFB64"},
		{Name: "cast5_ofb_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "OFB"},
		
		// Blowfish (key=16, iv=0 ou 8)
		{Name: "bf_ecb_cipher", KeySize: 16, IVSize: 0, BlockSize: 8, Mode: "ECB"},
		{Name: "bf_cbc_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "CBC"},
		{Name: "bf_cfb64_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "CFB64"},
		{Name: "bf_ofb_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "OFB"},
		
		// IDEA (key=16, iv=0 ou 8)
		{Name: "idea_ecb_cipher", KeySize: 16, IVSize: 0, BlockSize: 8, Mode: "ECB"},
		{Name: "idea_cbc_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "CBC"},
		{Name: "idea_cfb64_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "CFB64"},
		{Name: "idea_ofb_cipher", KeySize: 16, IVSize: 8, BlockSize: 8, Mode: "OFB"},
	}
}

// Identify tenta identificar o cipher baseado em key e IV sizes
func (h *CipherHeuristic) Identify(keyHex, ivHex string) (*CipherInfo, error) {
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decode key hex: %w", err)
	}
	
	var ivBytes []byte
	if ivHex != "" {
		ivBytes, err = hex.DecodeString(ivHex)
		if err != nil {
			return nil, fmt.Errorf("decode iv hex: %w", err)
		}
	}
	
	keySize := len(keyBytes)
	ivSize := len(ivBytes)
	
	var bestMatch *CipherInfo
	bestScore := 0.0
	
	for _, cipher := range h.knowCiphers {
		score := h.scoreCipher(cipher, keySize, ivSize)
		if score > bestScore {
			bestScore = score
			bestMatch = &cipher
		}
	}
	
	if bestMatch == nil || bestScore < 0.3 {
		return nil, fmt.Errorf("cipher não identificado (keySize=%d, ivSize=%d)", keySize, ivSize)
	}
	
	bestMatch.Confidence = bestScore
	return bestMatch, nil
}

// scoreCipher calcula score de correspondência
func (h *CipherHeuristic) scoreCipher(cipher CipherInfo, keySize, ivSize int) float64 {
	score := 0.0
	
	// Score para key size
	if cipher.KeySize == -1 {
		// RC4: key size variável (5-40 bytes)
		if keySize >= 5 && keySize <= 40 {
			score += 0.6
		}
	} else if cipher.KeySize == keySize {
		score += 0.6
	} else {
		// Penalidade por diferença de key size
		diff := keySize - cipher.KeySize
		if diff < 0 {
			diff = -diff
		}
		score -= float64(diff) * 0.2
	}
	
	// Score para IV size
	if cipher.IVSize == 0 {
		if ivSize == 0 {
			score += 0.4 // Stream cipher ou ECB sem IV
		} else {
			score -= 0.2 // Esperava sem IV mas tem
		}
	} else if cipher.IVSize == ivSize {
		score += 0.4
	} else {
		// Diferença de IV size
		diff := cipher.IVSize - ivSize
		if diff < 0 {
			diff = -diff
		}
		score -= float64(diff) * 0.1
	}
	
	return score
}

// GetAllCiphers retorna lista de ciphers conhecidos
func (h *CipherHeuristic) GetAllCiphers() []CipherInfo {
	return h.knowCiphers
}

// PrintCiphers imprime tabela de ciphers conhecidos
func (h *CipherHeuristic) PrintCiphers() {
	fmt.Println("=== Ciphers Conhecidos ===")
	fmt.Printf("%-30s %-10s %-10s %-10s %-8s\n", "Name", "KeySize", "IVSize", "Block", "Mode")
	fmt.Println(string(make([]byte, 70)))
	for _, c := range h.knowCiphers {
		fmt.Printf("%-30s %-10d %-10d %-10d %-8s\n", c.Name, c.KeySize, c.IVSize, c.BlockSize, c.Mode)
	}
}
