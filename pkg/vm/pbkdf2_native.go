package vm

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/sha3"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// pbkdf2State é o estado Go da classe tPBKDF2 (TDN: TLPP - Classes úteis /
// tPBKDF2, RFC2898 §5.2).
type pbkdf2State struct {
	password    string
	hasPassword bool
	salt        string
	hasSalt     bool
	iteration   int
	keyLength   int
	digest      string
	key         []byte
	lastErr     string
}

func newPBKDF2State() *pbkdf2State {
	return &pbkdf2State{iteration: 1000, keyLength: 16, digest: "SHA256"}
}

func newPBKDF2Object() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TPBKDF2", nil)
	obj.Native = newPBKDF2State()
	return obj
}

// pbkdf2HashFunc resolve o nome de digest documentado pela TDN para o
// construtor hash.Hash correspondente. SHA1/SHA224/SHA256/SHA384/SHA512/
// SHA512_224/SHA512_256 vêm da stdlib; SHA3_* de golang.org/x/crypto/sha3
// (já dependência transitiva do módulo).
func pbkdf2HashFunc(name string) func() hash.Hash {
	switch name {
	case "SHA1":
		return sha1.New
	case "SHA224":
		return sha256.New224
	case "SHA256":
		return sha256.New
	case "SHA384":
		return sha512.New384
	case "SHA512":
		return sha512.New
	case "SHA512_224":
		return sha512.New512_224
	case "SHA512_256":
		return sha512.New512_256
	case "SHA3_224":
		return sha3.New224
	case "SHA3_256":
		return sha3.New256
	case "SHA3_384":
		return sha3.New384
	case "SHA3_512":
		return sha3.New512
	default:
		return nil
	}
}

func (v *VM) callTPBKDF2Method(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*pbkdf2State)
	if !ok {
		return fmt.Errorf("tPBKDF2: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		v.push(obj)
		return nil

	case "SETPASSWORD":
		st.password = advplrt.ToString(getArg(args, 0))
		st.hasPassword = true
		v.push(advplrt.Nil)
		return nil

	case "SETSALT":
		st.salt = advplrt.ToString(getArg(args, 0))
		st.hasSalt = true
		v.push(advplrt.Nil)
		return nil

	case "SETITERATION":
		st.iteration = int(advplrt.ToFloat(getArg(args, 0)))
		v.push(advplrt.Nil)
		return nil

	case "SETKEYLENGTH":
		st.keyLength = int(advplrt.ToFloat(getArg(args, 0)))
		v.push(advplrt.Nil)
		return nil

	case "SETDIGEST":
		st.digest = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
		return nil

	case "ENCRYPT":
		st.key = nil
		if !st.hasPassword {
			st.lastErr = "PBKDF2 The password is not provided."
			v.push(advplrt.NewBool(false))
			return nil
		}
		if !st.hasSalt {
			st.lastErr = "PBKDF2 The salt is missing."
			v.push(advplrt.NewBool(false))
			return nil
		}
		if len(st.salt) > 64 {
			st.lastErr = "PBKDF2 The salt size is greater than 64, too large."
			v.push(advplrt.NewBool(false))
			return nil
		}
		if st.iteration < 1 {
			st.lastErr = "PBKDF2 The minimum number of iterations is 1."
			v.push(advplrt.NewBool(false))
			return nil
		}
		if st.keyLength < 8 {
			st.lastErr = "PBKDF2 The minimum key size is 8 bytes or 64 bits."
			v.push(advplrt.NewBool(false))
			return nil
		}
		if st.keyLength > 64 {
			st.lastErr = "PBKDF2 The maximum key size is 64 bytes or 512 bits."
			v.push(advplrt.NewBool(false))
			return nil
		}
		hf := pbkdf2HashFunc(st.digest)
		if hf == nil {
			st.lastErr = "PBKDF2 Invalid digest."
			v.push(advplrt.NewBool(false))
			return nil
		}
		st.key = pbkdf2.Key([]byte(st.password), []byte(st.salt), st.iteration, st.keyLength, hf)
		st.lastErr = ""
		v.push(advplrt.NewBool(true))
		return nil

	case "GETKEYHEX":
		v.push(advplrt.NewString(strings.ToUpper(hex.EncodeToString(st.key))))
		return nil

	case "GETKEYRAW":
		v.push(advplrt.NewString(string(st.key)))
		return nil

	case "GETKEYBASE64":
		v.push(advplrt.NewString(base64.StdEncoding.EncodeToString(st.key)))
		return nil

	case "GETKEYURL_BASE64":
		v.push(advplrt.NewString(base64.URLEncoding.EncodeToString(st.key)))
		return nil

	case "RELEASE":
		// TDN: "Obtém a versão da lib da openssl utilizada." AdvPP não usa
		// openssl (implementação Go pura via golang.org/x/crypto/pbkdf2) —
		// retorna essa identificação em vez de simular uma versão OpenSSL
		// que não existe neste runtime.
		v.push(advplrt.NewString("AdvPP (Go crypto/pbkdf2, sem OpenSSL)"))
		return nil

	case "GETLASTERROR":
		v.push(advplrt.NewString(st.lastErr))
		return nil

	default:
		return fmt.Errorf("unknown method %s on tPBKDF2", method)
	}
}
