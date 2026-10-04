package wire

import (
	"encoding/base64"
	"fmt"
)

// EvalStatus classifica o resultado textual de um DAP evaluate.
type EvalStatus int

const (
	EvalOK    EvalStatus = iota // base64 válido decodificado
	EvalNil                     // recurso ausente (NIL no servidor)
	EvalEmpty                   // binário vazio
)

func (s EvalStatus) String() string {
	switch s {
	case EvalOK:
		return "ok"
	case EvalNil:
		return "nil"
	case EvalEmpty:
		return "empty"
	}
	return "indefinido"
}

// UnquoteDAP remove aspas duplas externas quando presentes — o DAP devolve
// strings literais entre aspas (ex.: "\"aG9sYQ==\"").
func UnquoteDAP(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// DecodeEvalValue interpreta o resultado de
// Encode64(GetApoRes('<recurso>')) feito via DAP evaluate (porte fiel de
// decode_result, dap_proxy39.py — onda 39): NIL = recurso inexistente,
// vazio = binário vazio, senão base64.
func DecodeEvalValue(s string) ([]byte, EvalStatus, error) {
	s = UnquoteDAP(s)
	switch s {
	case "NIL":
		return nil, EvalNil, nil
	case "":
		return nil, EvalEmpty, nil
	}
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, 0, fmt.Errorf("base64: %w", err)
	}
	return data, EvalOK, nil
}
