package vm

import (
	"golang.org/x/crypto/argon2"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerArgon2Natives registra a função Argon2id (TDN: TLPP - Funções
// úteis / Argon2id, RFC9106).
//
// Mapeamento de parâmetros: a TDN documenta nThreads (núcleos de execução
// concorrente, "sempre menor que nLanes") separado de nLanes (em quantas
// partes a memória é dividida, "sempre maior que nThreads", afeta o hash
// final conforme RFC9106 §3.2 — é o parâmetro de paralelismo "p"). A API Go
// (golang.org/x/crypto/argon2) tem um único parâmetro `threads uint8` que
// corresponde ao "p" da RFC (afeta o digest, é o mesmo "lanes"). Por isso
// usamos nLanes como o `threads` passado a argon2.IDKey — é o valor que
// determina o hash — e nThreads é validado mas não tem efeito funcional
// (a implementação Go gerencia sua própria concorrência interna).
func registerArgon2Natives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// Argon2id(cText, cSalt, [nMemoryCost=65536], [nIterations=2], [nThreads=2], [nHashLen=128], [nLanes=nThreads]) -> cHashDigest
	natives["ARGON2ID"] = func(args []advplrt.Value) (advplrt.Value, error) {
		text := getArgString(args, 0, "")
		salt := getArgString(args, 1, "")
		if text == "" || salt == "" {
			return advplrt.NewString(""), nil
		}

		memoryCost := uint32(65536)
		if len(args) > 2 && args[2] != nil && args[2] != advplrt.Nil {
			memoryCost = uint32(advplrt.ToFloat(args[2]))
		}
		iterations := uint32(2)
		if len(args) > 3 && args[3] != nil && args[3] != advplrt.Nil {
			iterations = uint32(advplrt.ToFloat(args[3]))
		}
		threads := uint32(2)
		if len(args) > 4 && args[4] != nil && args[4] != advplrt.Nil {
			threads = uint32(advplrt.ToFloat(args[4]))
		}
		hashLen := uint32(128)
		if len(args) > 5 && args[5] != nil && args[5] != advplrt.Nil {
			hashLen = uint32(advplrt.ToFloat(args[5]))
		}
		lanes := threads
		if len(args) > 6 && args[6] != nil && args[6] != advplrt.Nil {
			lanes = uint32(advplrt.ToFloat(args[6]))
		}
		if memoryCost == 0 || iterations == 0 || hashLen == 0 || lanes == 0 || lanes > 255 {
			return advplrt.NewString(""), nil
		}

		digest := argon2.IDKey([]byte(text), []byte(salt), iterations, memoryCost, uint8(lanes), hashLen)
		return advplrt.NewString(string(digest)), nil
	}
}
