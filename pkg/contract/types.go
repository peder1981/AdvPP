package contract

import (
	"crypto/sha256"
	"encoding/hex"
)

// Contract represents a deployed contract on the network.
type Contract struct {
	Code     []byte  // WASM bytecode
	CodeHash string  // SHA256(Code) hex
	Params   []byte  // opaque parameters
	Key      string  // H(H(Code) || Params) - location-independent key
	Location float64 // ℓ(C) ∈ [0,1)
}

// State is the current public state of a contract.
type State struct {
	Data      []byte // contract-defined byte string
	Signature string // Ed25519 signature
	Version   uint64 // logical clock
}

// Update is an operation submitted to a contract.
type Update struct {
	Data      []byte // contract-defined update payload
	Signature string // signed by peer
	Timestamp int64  // unix nanoseconds
}

// NewContract creates a contract from WASM code and params.
func NewContract(code, params []byte) *Contract {
	codeHash := hashBytes(code)
	keyStr := hashBytes(append([]byte(codeHash), params...))
	location := hashToLocation(keyStr)

	return &Contract{
		Code:     code,
		CodeHash: codeHash,
		Params:   params,
		Key:      keyStr,
		Location: location,
	}
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hashToLocation(keyStr string) float64 {
	h := sha256.Sum256([]byte(keyStr))
	sum := uint64(0)
	for i := 0; i < 8; i++ {
		sum = sum*256 + uint64(h[i])
	}
	return float64(sum) / float64(^uint64(0))
}
