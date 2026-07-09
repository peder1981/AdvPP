package vm

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/advpl/compiler/pkg/mvc"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

func (v *VM) registerNatives() {
	natives := map[string]func(args []advplrt.Value) (advplrt.Value, error){

		// --- Output ---
		"CONOUT": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := buildOutputString(args)
			fmt.Println(msg)
			v.writeOut(msg)
			v.output.WriteString(msg + "\n")
			return advplrt.Nil, nil
		},
		"CONOUTW": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := buildOutputString(args)
			fmt.Println(msg)
			v.writeOut(msg)
			v.output.WriteString(msg + "\n")
			return advplrt.Nil, nil
		},
		"ALERT": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := getArgString(args, 0, "")
			if v.uiProvider != nil {
				v.uiProvider.MsgAlert(msg, "Alert")
			} else {
				fmt.Println("[ALERT] " + msg)
			}
			return advplrt.Nil, nil
		},

		// --- Dialogs ---
		"MSGINFO": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := getArgString(args, 0, "")
			title := getArgString(args, 1, "Information")
			if v.uiProvider != nil {
				v.uiProvider.MsgInfo(msg, title)
			} else {
				fmt.Printf("[INFO] %s: %s\n", title, msg)
			}
			return advplrt.Nil, nil
		},
		"MSGSTOP": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := getArgString(args, 0, "")
			title := getArgString(args, 1, "Stop")
			if v.uiProvider != nil {
				v.uiProvider.MsgStop(msg, title)
			} else {
				fmt.Printf("[STOP] %s: %s\n", title, msg)
			}
			return advplrt.Nil, nil
		},
		"MSGALERT": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := getArgString(args, 0, "")
			title := getArgString(args, 1, "Alert")
			if v.uiProvider != nil {
				v.uiProvider.MsgAlert(msg, title)
			} else {
				fmt.Printf("[ALERT] %s: %s\n", title, msg)
			}
			return advplrt.Nil, nil
		},
		"MSGYESNO": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := getArgString(args, 0, "")
			title := getArgString(args, 1, "Confirm")
			if v.uiProvider != nil {
				return advplrt.NewBool(v.uiProvider.MsgYesNo(msg, title)), nil
			}
			return advplrt.True, nil
		},

		// --- String functions ---
		"ALLTRIM": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(strings.TrimSpace(advplrt.ToString(getArg(args, 0)))), nil
		},
		"LTRIM": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(strings.TrimLeft(advplrt.ToString(getArg(args, 0)), " \t\r\n")), nil
		},
		"RTRIM": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(strings.TrimRight(advplrt.ToString(getArg(args, 0)), " \t\r\n")), nil
		},
		"TRIM": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(strings.TrimRight(advplrt.ToString(getArg(args, 0)), " \t\r\n")), nil
		},
		"STR": func(args []advplrt.Value) (advplrt.Value, error) {
			val := advplrt.ToFloat(getArg(args, 0))
			decimals := int(advplrt.ToFloat(getArg(args, 2)))
			if decimals < 0 {
				decimals = 0
			}
			if len(args) >= 3 {
				return advplrt.NewString(strconv.FormatFloat(val, 'f', decimals, 64)), nil
			}
			if val == math.Trunc(val) {
				return advplrt.NewString(strconv.FormatInt(int64(val), 10)), nil
			}
			return advplrt.NewString(strconv.FormatFloat(val, 'f', -1, 64)), nil
		},
		"STRTRAN": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			search := advplrt.ToString(getArg(args, 1))
			repl := ""
			if len(args) >= 3 {
				repl = advplrt.ToString(getArg(args, 2))
			}
			return advplrt.NewString(strings.ReplaceAll(s, search, repl)), nil
		},
		"STRZERO": func(args []advplrt.Value) (advplrt.Value, error) {
			val := int(advplrt.ToFloat(getArg(args, 0)))
			size := int(advplrt.ToFloat(getArg(args, 1)))
			s := strconv.Itoa(val)
			if len(s) < size {
				s = strings.Repeat("0", size-len(s)) + s
			}
			return advplrt.NewString(s), nil
		},
		"SUBSTR": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			start := int(advplrt.ToFloat(getArg(args, 1)))
			if start < 1 {
				start = 1
			}
			if len(args) >= 3 {
				length := int(advplrt.ToFloat(getArg(args, 2)))
				if start > len(s) {
					return advplrt.NewString(""), nil
				}
				end := start - 1 + length
				if end > len(s) {
					end = len(s)
				}
				return advplrt.NewString(s[start-1 : end]), nil
			}
			if start > len(s) {
				return advplrt.NewString(""), nil
			}
			return advplrt.NewString(s[start-1:]), nil
		},
		"STUFF": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			start := int(advplrt.ToFloat(getArg(args, 1)))
			count := int(advplrt.ToFloat(getArg(args, 2)))
			repl := advplrt.ToString(getArg(args, 3))
			if start < 1 {
				start = 1
			}
			if start > len(s) {
				return advplrt.NewString(s + repl), nil
			}
			end := start - 1 + count
			if end > len(s) {
				end = len(s)
			}
			return advplrt.NewString(s[:start-1] + repl + s[end:]), nil
		},
		"LEN": func(args []advplrt.Value) (advplrt.Value, error) {
			val := getArg(args, 0)
			if s, ok := val.(*advplrt.StringValue); ok {
				return advplrt.NewNumber(float64(len(s.Val))), nil
			}
			if a, ok := val.(*advplrt.ArrayValue); ok {
				return advplrt.NewNumber(float64(len(a.Elements))), nil
			}
			return advplrt.NewNumber(0), nil
		},
		"AT": func(args []advplrt.Value) (advplrt.Value, error) {
			search := advplrt.ToString(getArg(args, 0))
			s := advplrt.ToString(getArg(args, 1))
			idx := strings.Index(s, search)
			if idx == -1 {
				return advplrt.NewNumber(0), nil
			}
			return advplrt.NewNumber(float64(idx + 1)), nil
		},
		"RAT": func(args []advplrt.Value) (advplrt.Value, error) {
			search := advplrt.ToString(getArg(args, 0))
			s := advplrt.ToString(getArg(args, 1))
			idx := strings.LastIndex(s, search)
			if idx == -1 {
				return advplrt.NewNumber(0), nil
			}
			return advplrt.NewNumber(float64(idx + 1)), nil
		},
		"UPPER": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(strings.ToUpper(advplrt.ToString(getArg(args, 0)))), nil
		},
		"LOWER": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(strings.ToLower(advplrt.ToString(getArg(args, 0)))), nil
		},
		"PADC": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			size := int(advplrt.ToFloat(getArg(args, 1)))
			pad := " "
			if len(args) >= 3 {
				pad = advplrt.ToString(getArg(args, 2))
			}
			if pad == "" {
				pad = " "
			}
			for len(s) < size {
				s = pad + s + pad
				if len(s) > size {
					s = s[:size]
				}
			}
			return advplrt.NewString(s), nil
		},
		"PADL": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			size := int(advplrt.ToFloat(getArg(args, 1)))
			pad := " "
			if len(args) >= 3 {
				pad = advplrt.ToString(getArg(args, 2))
			}
			if pad == "" {
				pad = " "
			}
			for len(s) < size {
				s = pad + s
			}
			if len(s) > size {
				s = s[len(s)-size:]
			}
			return advplrt.NewString(s), nil
		},
		"PADR": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			size := int(advplrt.ToFloat(getArg(args, 1)))
			pad := " "
			if len(args) >= 3 {
				pad = advplrt.ToString(getArg(args, 2))
			}
			if pad == "" {
				pad = " "
			}
			for len(s) < size {
				s = s + pad
			}
			if len(s) > size {
				s = s[:size]
			}
			return advplrt.NewString(s), nil
		},
		"CHR": func(args []advplrt.Value) (advplrt.Value, error) {
			code := int(advplrt.ToFloat(getArg(args, 0)))
			return advplrt.NewString(string(rune(code))), nil
		},
		"ASC": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if len(s) == 0 {
				return advplrt.NewNumber(0), nil
			}
			return advplrt.NewNumber(float64(s[0])), nil
		},
		"VAL": func(args []advplrt.Value) (advplrt.Value, error) {
			s := strings.TrimSpace(advplrt.ToString(getArg(args, 0)))
			f, _ := strconv.ParseFloat(s, 64)
			return advplrt.NewNumber(f), nil
		},
		"CVALTOCHAR": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(advplrt.ToString(getArg(args, 0))), nil
		},
		"CTOD": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			t, err := time.Parse("02/01/2006", s)
			if err != nil {
				return advplrt.Nil, nil
			}
			return advplrt.NewDate(t), nil
		},
		"DTOS": func(args []advplrt.Value) (advplrt.Value, error) {
			if d, ok := getArg(args, 0).(*advplrt.DateValue); ok {
				return advplrt.NewString(d.Val.Format("20060102")), nil
			}
			return advplrt.NewString(""), nil
		},
		"DTOC": func(args []advplrt.Value) (advplrt.Value, error) {
			if d, ok := getArg(args, 0).(*advplrt.DateValue); ok {
				return advplrt.NewString(d.Val.Format("02/01/2006")), nil
			}
			return advplrt.NewString(""), nil
		},
		"TRANSFORM": func(args []advplrt.Value) (advplrt.Value, error) {
			val := getArg(args, 0)
			mask := advplrt.ToString(getArg(args, 1))
			return advplrt.NewString(applyTransform(val, mask)), nil
		},
		"ISDIGIT": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if len(s) == 0 {
				return advplrt.False, nil
			}
			return advplrt.NewBool(unicode.IsDigit(rune(s[0]))), nil
		},
		"ISALPHA": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if len(s) == 0 {
				return advplrt.False, nil
			}
			return advplrt.NewBool(unicode.IsLetter(rune(s[0]))), nil
		},
		"ISLOWER": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if len(s) == 0 {
				return advplrt.False, nil
			}
			return advplrt.NewBool(unicode.IsLower(rune(s[0]))), nil
		},
		"ISUPPER": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if len(s) == 0 {
				return advplrt.False, nil
			}
			return advplrt.NewBool(unicode.IsUpper(rune(s[0]))), nil
		},
		"EMPTY": func(args []advplrt.Value) (advplrt.Value, error) {
			val := getArg(args, 0)
			if advplrt.IsNil(val) {
				return advplrt.True, nil
			}
			return advplrt.NewBool(!val.IsTruthy()), nil
		},
		"SPACE": func(args []advplrt.Value) (advplrt.Value, error) {
			n := int(advplrt.ToFloat(getArg(args, 0)))
			return advplrt.NewString(strings.Repeat(" ", n)), nil
		},
		"REPLICATE": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			n := int(advplrt.ToFloat(getArg(args, 1)))
			return advplrt.NewString(strings.Repeat(s, n)), nil
		},
		"STRTOKARR": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			delim := advplrt.ToString(getArg(args, 1))
			parts := strings.Split(s, delim)
			elems := make([]advplrt.Value, len(parts))
			for i, p := range parts {
				elems[i] = advplrt.NewString(p)
			}
			return advplrt.NewArray(elems), nil
		},
		"LEFT": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			count := int(advplrt.ToFloat(getArg(args, 1)))
			if count < 0 {
				count = 0
			}
			if count > len(s) {
				count = len(s)
			}
			return advplrt.NewString(s[:count]), nil
		},
		"RIGHT": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			count := int(advplrt.ToFloat(getArg(args, 1)))
			if count < 0 {
				count = 0
			}
			if count > len(s) {
				count = len(s)
			}
			return advplrt.NewString(s[len(s)-count:]), nil
		},
		"REPLICA": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			n := int(advplrt.ToFloat(getArg(args, 1)))
			return advplrt.NewString(strings.Repeat(s, n)), nil
		},
		"CAPSLOCK": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if len(s) == 0 {
				return advplrt.NewString(""), nil
			}
			return advplrt.NewString(strings.ToUpper(string(s[0])) + strings.ToLower(s[1:])), nil
		},
		"PROPER": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			words := strings.Fields(s)
			for i, word := range words {
				if len(word) > 0 {
					words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
				}
			}
			return advplrt.NewString(strings.Join(words, " ")), nil
		},
		"ATC": func(args []advplrt.Value) (advplrt.Value, error) {
			search := strings.ToLower(advplrt.ToString(getArg(args, 0)))
			s := strings.ToLower(advplrt.ToString(getArg(args, 1)))
			idx := strings.Index(s, search)
			if idx == -1 {
				return advplrt.NewNumber(0), nil
			}
			return advplrt.NewNumber(float64(idx + 1)), nil
		},
		"RATC": func(args []advplrt.Value) (advplrt.Value, error) {
			search := strings.ToLower(advplrt.ToString(getArg(args, 0)))
			s := strings.ToLower(advplrt.ToString(getArg(args, 1)))
			idx := strings.LastIndex(s, search)
			if idx == -1 {
				return advplrt.NewNumber(0), nil
			}
			return advplrt.NewNumber(float64(idx + 1)), nil
		},
		"GETWORDNUM": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			wordNum := int(advplrt.ToFloat(getArg(args, 1)))
			delim := " "
			if len(args) >= 3 {
				delim = advplrt.ToString(getArg(args, 2))
			}
			words := strings.Split(s, delim)
			if wordNum < 1 || wordNum > len(words) {
				return advplrt.NewString(""), nil
			}
			return advplrt.NewString(words[wordNum-1]), nil
		},
		"WORDS": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			delim := " "
			if len(args) >= 2 {
				delim = advplrt.ToString(getArg(args, 1))
			}
			words := strings.Split(s, delim)
			return advplrt.NewNumber(float64(len(words))), nil
		},
		"FILENOEXT": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if idx := strings.LastIndex(s, "."); idx != -1 {
				return advplrt.NewString(s[:idx]), nil
			}
			return advplrt.NewString(s), nil
		},
		"FILEEXT": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if idx := strings.LastIndex(s, "."); idx != -1 {
				return advplrt.NewString(s[idx+1:]), nil
			}
			return advplrt.NewString(""), nil
		},
		"FILENAME": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if idx := strings.LastIndex(s, "/"); idx != -1 {
				return advplrt.NewString(s[idx+1:]), nil
			}
			if idx := strings.LastIndex(s, "\\"); idx != -1 {
				return advplrt.NewString(s[idx+1:]), nil
			}
			return advplrt.NewString(s), nil
		},
		"FILEPATH": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if idx := strings.LastIndex(s, "/"); idx != -1 {
				return advplrt.NewString(s[:idx+1]), nil
			}
			if idx := strings.LastIndex(s, "\\"); idx != -1 {
				return advplrt.NewString(s[:idx+1]), nil
			}
			return advplrt.NewString(""), nil
		},
		"FILEDIR": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			if idx := strings.LastIndex(s, "/"); idx != -1 {
				return advplrt.NewString(s[:idx]), nil
			}
			if idx := strings.LastIndex(s, "\\"); idx != -1 {
				return advplrt.NewString(s[:idx]), nil
			}
			return advplrt.NewString(""), nil
		},

		// --- Numeric functions ---
		"ABS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Abs(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"INT": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(float64(int64(advplrt.ToFloat(getArg(args, 0))))), nil
		},
		"ROUND": func(args []advplrt.Value) (advplrt.Value, error) {
			val := advplrt.ToFloat(getArg(args, 0))
			decimals := int(advplrt.ToFloat(getArg(args, 1)))
			pow := math.Pow(10, float64(decimals))
			return advplrt.NewNumber(math.Round(val*pow) / pow), nil
		},
		"NOROUND": func(args []advplrt.Value) (advplrt.Value, error) {
			val := advplrt.ToFloat(getArg(args, 0))
			decimals := int(advplrt.ToFloat(getArg(args, 1)))
			pow := math.Pow(10, float64(decimals))
			return advplrt.NewNumber(math.Trunc(val*pow) / pow), nil
		},
		"CEILING": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Ceil(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"FLOOR": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Floor(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"MOD": func(args []advplrt.Value) (advplrt.Value, error) {
			a := advplrt.ToFloat(getArg(args, 0))
			b := advplrt.ToFloat(getArg(args, 1))
			if b == 0 {
				return advplrt.NewNumber(0), nil
			}
			return advplrt.NewNumber(math.Mod(a, b)), nil
		},
		"MAX": func(args []advplrt.Value) (advplrt.Value, error) {
			if len(args) == 0 {
				return advplrt.Nil, nil
			}
			result := args[0]
			for _, arg := range args[1:] {
				if advplrt.ToFloat(arg) > advplrt.ToFloat(result) {
					result = arg
				}
			}
			return result, nil
		},
		"MIN": func(args []advplrt.Value) (advplrt.Value, error) {
			if len(args) == 0 {
				return advplrt.Nil, nil
			}
			result := args[0]
			for _, arg := range args[1:] {
				if advplrt.ToFloat(arg) < advplrt.ToFloat(result) {
					result = arg
				}
			}
			return result, nil
		},
		"SQRT": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Sqrt(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"EXP": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Exp(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"LOG": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Log(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"RANDOM": func(args []advplrt.Value) (advplrt.Value, error) {
			max := int(advplrt.ToFloat(getArg(args, 0)))
			if max <= 0 {
				max = 100
			}
			return advplrt.NewNumber(float64(rand.Intn(max) + 1)), nil
		},
		"SIGN": func(args []advplrt.Value) (advplrt.Value, error) {
			val := advplrt.ToFloat(getArg(args, 0))
			if val > 0 {
				return advplrt.NewNumber(1), nil
			} else if val < 0 {
				return advplrt.NewNumber(-1), nil
			}
			return advplrt.NewNumber(0), nil
		},
		"POWER": func(args []advplrt.Value) (advplrt.Value, error) {
			base := advplrt.ToFloat(getArg(args, 0))
			exp := advplrt.ToFloat(getArg(args, 1))
			return advplrt.NewNumber(math.Pow(base, exp)), nil
		},
		"PI": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Pi), nil
		},
		"SIN": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Sin(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"COS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Cos(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"TAN": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Tan(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"ASIN": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Asin(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"ACOS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Acos(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"ATAN": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(math.Atan(advplrt.ToFloat(getArg(args, 0)))), nil
		},
		"DEG2RAD": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(advplrt.ToFloat(getArg(args, 0)) * math.Pi / 180), nil
		},
		"RAD2DEG": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(advplrt.ToFloat(getArg(args, 0)) * 180 / math.Pi), nil
		},

		// --- Date functions ---
		"DATE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewDate(time.Now()), nil
		},
		// Array(n1[, n2, ...]) builds a NIL-filled array; extra dimensions
		// nest arrays-of-arrays (Array(3,2) => 3 elements, each a 2-array).
		"ARRAY": func(args []advplrt.Value) (advplrt.Value, error) {
			return makeArray(args), nil
		},
		"DAY": func(args []advplrt.Value) (advplrt.Value, error) {
			if d, ok := getArg(args, 0).(*advplrt.DateValue); ok {
				return advplrt.NewNumber(float64(d.Val.Day())), nil
			}
			return advplrt.NewNumber(0), nil
		},
		"MONTH": func(args []advplrt.Value) (advplrt.Value, error) {
			if d, ok := getArg(args, 0).(*advplrt.DateValue); ok {
				return advplrt.NewNumber(float64(d.Val.Month())), nil
			}
			return advplrt.NewNumber(0), nil
		},
		"YEAR": func(args []advplrt.Value) (advplrt.Value, error) {
			if d, ok := getArg(args, 0).(*advplrt.DateValue); ok {
				return advplrt.NewNumber(float64(d.Val.Year())), nil
			}
			return advplrt.NewNumber(0), nil
		},
		"CMONTH": func(args []advplrt.Value) (advplrt.Value, error) {
			if d, ok := getArg(args, 0).(*advplrt.DateValue); ok {
				months := []string{"January", "February", "March", "April", "May", "June",
					"July", "August", "September", "October", "November", "December"}
				return advplrt.NewString(months[d.Val.Month()-1]), nil
			}
			return advplrt.NewString(""), nil
		},
		"CDOW": func(args []advplrt.Value) (advplrt.Value, error) {
			if d, ok := getArg(args, 0).(*advplrt.DateValue); ok {
				days := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
				return advplrt.NewString(days[d.Val.Weekday()]), nil
			}
			return advplrt.NewString(""), nil
		},
		"DOW": func(args []advplrt.Value) (advplrt.Value, error) {
			if d, ok := getArg(args, 0).(*advplrt.DateValue); ok {
				return advplrt.NewNumber(float64(d.Val.Weekday() + 1)), nil
			}
			return advplrt.NewNumber(0), nil
		},
		"TIME": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(time.Now().Format("15:04:05")), nil
		},
		"SECONDS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(float64(time.Now().UnixNano()) / 1e9), nil
		},
		"STOD": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			t, err := time.Parse("20060102", s)
			if err != nil {
				return advplrt.Nil, nil
			}
			return advplrt.NewDate(t), nil
		},
		"ELAPTIME": func(args []advplrt.Value) (advplrt.Value, error) {
			t1 := advplrt.ToFloat(getArg(args, 0))
			t2 := advplrt.ToFloat(getArg(args, 1))
			return advplrt.NewNumber(t2 - t1), nil
		},
		"CTOT": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			t, err := time.Parse("15:04:05", s)
			if err != nil {
				return advplrt.Nil, nil
			}
			return advplrt.NewNumber(float64(t.Hour()*3600 + t.Minute()*60 + t.Second())), nil
		},
		"TTOC": func(args []advplrt.Value) (advplrt.Value, error) {
			seconds := advplrt.ToFloat(getArg(args, 0))
			hours := int(seconds) / 3600
			minutes := (int(seconds) % 3600) / 60
			secs := int(seconds) % 60
			return advplrt.NewString(fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)), nil
		},

		// --- Array functions ---
		"AADD": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				var val advplrt.Value = advplrt.Nil
				if len(args) >= 2 {
					val = getArg(args, 1)
				}
				a.Elements = append(a.Elements, val)
				return val, nil
			}
			return advplrt.Nil, nil
		},
		"ASIZE": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				size := int(advplrt.ToFloat(getArg(args, 1)))
				if size < len(a.Elements) {
					a.Elements = a.Elements[:size]
				} else {
					for len(a.Elements) < size {
						a.Elements = append(a.Elements, advplrt.Nil)
					}
				}
			}
			return advplrt.Nil, nil
		},
		"ASCAN": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				search := getArg(args, 1)
				for i, elem := range a.Elements {
					if elem.Equals(search) {
						return advplrt.NewNumber(float64(i + 1)), nil
					}
				}
			}
			return advplrt.NewNumber(0), nil
		},
		"ADEL": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				idx := int(advplrt.ToFloat(getArg(args, 1)))
				if idx >= 1 && idx <= len(a.Elements) {
					a.Elements = append(a.Elements[:idx-1], a.Elements[idx:]...)
				}
			}
			return advplrt.Nil, nil
		},
		"AINS": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				idx := int(advplrt.ToFloat(getArg(args, 1)))
				if idx >= 1 && idx <= len(a.Elements)+1 {
					a.Elements = append(a.Elements, advplrt.Nil)
					copy(a.Elements[idx:], a.Elements[idx-1:])
					a.Elements[idx-1] = advplrt.Nil
				}
			}
			return advplrt.Nil, nil
		},
		"ALEN": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				return advplrt.NewNumber(float64(len(a.Elements))), nil
			}
			return advplrt.NewNumber(0), nil
		},
		"ACLONE": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				elems := make([]advplrt.Value, len(a.Elements))
				copy(elems, a.Elements)
				return advplrt.NewArray(elems), nil
			}
			return advplrt.NewArray([]advplrt.Value{}), nil
		},
		"AFILL": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				val := getArg(args, 1)
				for i := range a.Elements {
					a.Elements[i] = val
				}
			}
			return advplrt.Nil, nil
		},
		"ASORT": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				sortValues(a.Elements)
			}
			return advplrt.Nil, nil
		},
		"AEVAL": func(args []advplrt.Value) (advplrt.Value, error) {
			if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok {
				_ = a
			}
			return advplrt.Nil, nil
		},

		// --- Logic / Type ---
		"IIF": func(args []advplrt.Value) (advplrt.Value, error) {
			if len(args) < 3 {
				return advplrt.Nil, nil
			}
			if args[0].IsTruthy() {
				return args[1], nil
			}
			return args[2], nil
		},
		"IF": func(args []advplrt.Value) (advplrt.Value, error) {
			if len(args) < 3 {
				return advplrt.Nil, nil
			}
			if args[0].IsTruthy() {
				return args[1], nil
			}
			return args[2], nil
		},
		"VALTYPE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(advplrt.ValType(getArg(args, 0))), nil
		},
		"TYPE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(advplrt.ValType(getArg(args, 0))), nil
		},
		"ISNIL": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(advplrt.IsNil(getArg(args, 0))), nil
		},

		// --- Error ---
		"USEREXCEPTION": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := advplrt.ToString(getArg(args, 0))
			return nil, advplrt.NewError(msg)
		},
		"THROW": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := advplrt.ToString(getArg(args, 0))
			return nil, advplrt.NewError(msg)
		},

		// --- Misc ---
		// StartJob(cFunc, cEnv, lWait, params...) — executa a função em um
		// VM isolado (semântica de work process do Protheus). cEnv é aceito
		// e ignorado (não há multi-ambiente neste runtime).
		"STARTJOB": func(args []advplrt.Value) (advplrt.Value, error) {
			funcName := advplrt.ToString(getArg(args, 0))
			if funcName == "" {
				return advplrt.False, fmt.Errorf("StartJob: missing function name")
			}
			wait := advplrt.ToBool(getArg(args, 2))
			var params []advplrt.Value
			if len(args) > 3 {
				params = args[3:]
			}
			if err := v.StartJob(funcName, wait, params); err != nil {
				return advplrt.False, err
			}
			return advplrt.True, nil
		},
		"SLEEP": func(args []advplrt.Value) (advplrt.Value, error) {
			ms := int(advplrt.ToFloat(getArg(args, 0)))
			time.Sleep(time.Duration(ms) * time.Millisecond)
			return advplrt.Nil, nil
		},
		"PROCNAME": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},
		"PROCLINE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(0), nil
		},
		"GETMV": func(args []advplrt.Value) (advplrt.Value, error) {
			return getArg(args, 1), nil
		},
		"GETNEWPAR": func(args []advplrt.Value) (advplrt.Value, error) {
			return getArg(args, 1), nil
		},
		"GETENV": func(args []advplrt.Value) (advplrt.Value, error) {
			name := advplrt.ToString(getArg(args, 0))
			return advplrt.NewString(getEnvOrDefault(name, advplrt.ToString(getArg(args, 1)))), nil
		},
		"FILE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.False, nil
		},
		"MAKEDIR": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"CURDIR": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("./"), nil
		},
		"GETSRVNAME": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("localhost"), nil
		},

		// --- Database stubs ---
		"DBSELECTAREA": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				alias := advplrt.ToString(getArg(args, 0))
				return advplrt.Nil, v.dbEngine.SelectArea(alias)
			}
			return advplrt.Nil, nil
		},
		"DBSEEK": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				key := advplrt.ToString(getArg(args, 0))
				found, err := v.dbEngine.Seek(key)
				if err != nil {
					return advplrt.False, err
				}
				return advplrt.NewBool(found), nil
			}
			return advplrt.False, nil
		},
		"DBSKIP": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				count := 1
				if len(args) >= 1 {
					count = int(advplrt.ToFloat(getArg(args, 0)))
				}
				return advplrt.Nil, v.dbEngine.Skip(count)
			}
			return advplrt.Nil, nil
		},
		"DBGOTOP": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				return advplrt.Nil, v.dbEngine.GoTop()
			}
			return advplrt.Nil, nil
		},
		"DBGOBOTTOM": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				return advplrt.Nil, v.dbEngine.GoBottom()
			}
			return advplrt.Nil, nil
		},
		"EOF": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				return advplrt.NewBool(v.dbEngine.EOF()), nil
			}
			return advplrt.True, nil
		},
		"BOF": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				return advplrt.NewBool(v.dbEngine.BOF()), nil
			}
			return advplrt.True, nil
		},
		"RECLOCK": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				return advplrt.NewBool(true), v.dbEngine.RecLock()
			}
			return advplrt.True, nil
		},
		"MSUNLOCK": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				return advplrt.Nil, v.dbEngine.MsUnlock()
			}
			return advplrt.Nil, nil
		},
		"RECCOUNT": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				return advplrt.NewNumber(float64(v.dbEngine.RecCount())), nil
			}
			return advplrt.NewNumber(0), nil
		},
		"RECNO": func(args []advplrt.Value) (advplrt.Value, error) {
			if v.dbEngine != nil {
				return advplrt.NewNumber(float64(v.dbEngine.RecNo())), nil
			}
			return advplrt.NewNumber(0), nil
		},
		"DBCLOSEAREA": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"DBSETORDER": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"DBSETFILTER": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"DBCLEARFILTER": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"DBAPPEND": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"DBDELETE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"DBCOMMIT": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"SELECT": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(0), nil
		},
		"ALIAS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},
		"USED": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.False, nil
		},
		"FIELDGET": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FIELDPUT": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FIELDPOS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(0), nil
		},
		"FIELDNAME": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},
		"XFILIAL": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},

		// --- MVC ---
		"FWFORMMODEL": func(args []advplrt.Value) (advplrt.Value, error) {
			name := advplrt.ToString(getArg(args, 0))
			model := mvc.NewFWFormModel(name)
			modelID := v.registerMVCModel(model)
			obj := advplrt.NewObject("FWFormModel", nil)
			obj.Props["name"] = advplrt.NewString(name)
			obj.Props["_modelId"] = advplrt.NewNumber(float64(modelID))
			return obj, nil
		},
		"FWFORMVIEW": func(args []advplrt.Value) (advplrt.Value, error) {
			name := advplrt.ToString(getArg(args, 0))
			modelArg := getArg(args, 1)
			var model *mvc.FWFormModel
			if modelObj, ok := modelArg.(*advplrt.ObjectValue); ok {
				if id, ok := modelObj.Props["_modelId"].(*advplrt.NumberValue); ok {
					model = v.getMVCModel(int(id.Val))
				}
			}
			view := mvc.NewFWFormView(name, model)
			viewID := v.registerMVCView(view)
			obj := advplrt.NewObject("FWFormView", nil)
			obj.Props["name"] = advplrt.NewString(name)
			obj.Props["_viewId"] = advplrt.NewNumber(float64(viewID))
			return obj, nil
		},
		"FWFORMBROWSE": func(args []advplrt.Value) (advplrt.Value, error) {
			name := advplrt.ToString(getArg(args, 0))
			modelArg := getArg(args, 1)
			var model *mvc.FWFormModel
			if modelObj, ok := modelArg.(*advplrt.ObjectValue); ok {
				if id, ok := modelObj.Props["_modelId"].(*advplrt.NumberValue); ok {
					model = v.getMVCModel(int(id.Val))
				}
			}
			browse := mvc.NewFWFormBrowse(name, model)
			browseID := v.registerMVCBrowse(browse)
			obj := advplrt.NewObject("FWFormBrowse", nil)
			obj.Props["name"] = advplrt.NewString(name)
			obj.Props["_browseId"] = advplrt.NewNumber(float64(browseID))
			return obj, nil
		},
		"FWFORMSTRUCT": func(args []advplrt.Value) (advplrt.Value, error) {
			obj := advplrt.NewObject("FWFormStruct", nil)
			return obj, nil
		},
		"FWMBROWSE": func(args []advplrt.Value) (advplrt.Value, error) {
			return newBrowseObject(), nil
		},
		"VIEWDEF": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"AXCADASTRO": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},

		// --- JSON ---
		"JSONOBJECT": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewObject("JsonObject", nil), nil
		},

		// --- Help ---
		"HELP": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := ""
			if len(args) >= 3 {
				msg = advplrt.ToString(getArg(args, 2))
			}
			fmt.Printf("[HELP] %s\n", msg)
			return advplrt.Nil, nil
		},

		// --- Set ---
		"SETDATE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"SETCENT": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"SET": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},

		// --- FreeObj ---
		"FREEOBJ": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWFREEOBJ": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWFREEARRAY": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWFREEVAR": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWINPUTBOX": func(args []advplrt.Value) (advplrt.Value, error) {
			defaultValue := getArgString(args, 2, "")
			return advplrt.NewString(defaultValue), nil
		},
		"FWHTTPENCODE": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			// Basic URL encoding
			encoded := ""
			for _, c := range s {
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
					encoded += string(c)
				} else {
					encoded += fmt.Sprintf("%%%02X", c)
				}
			}
			return advplrt.NewString(encoded), nil
		},
		"FW8601TODATE": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return advplrt.Nil, nil
			}
			return advplrt.NewDate(t), nil
		},
		"FWDATETO8601": func(args []advplrt.Value) (advplrt.Value, error) {
			if d, ok := getArg(args, 0).(*advplrt.DateValue); ok {
				return advplrt.NewString(d.Val.Format(time.RFC3339)), nil
			}
			return advplrt.NewString(""), nil
		},
		"FWGETUSERNAME": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("USER"), nil
		},
		"FWRETIDIOM": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("PORTUGUESE"), nil
		},
		"MSRETPATH": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("./"), nil
		},
		"USRRETNAME": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("USER"), nil
		},
		"FWALIASINDIC": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(false), nil
		},
		"FWMODEACCESS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewNumber(1), nil
		},
		"FWHASACCMODE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"FWURIDECODE": func(args []advplrt.Value) (advplrt.Value, error) {
			s := advplrt.ToString(getArg(args, 0))
			return advplrt.NewString(s), nil
		},
		"FWLOADSM0": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"FWJOINFILIAL": func(args []advplrt.Value) (advplrt.Value, error) {
			field := advplrt.ToString(getArg(args, 0))
			alias := advplrt.ToString(getArg(args, 1))
			return advplrt.NewString(field + "_" + alias), nil
		},
		"FWRESTAREA": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWGETAREA": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},
		"FWAPPSTACK": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},
		"FWCALLAPP": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWLIBVERSION": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("1.0.0"), nil
		},
		"FWLISTBRANCHES": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewArray([]advplrt.Value{}), nil
		},
		"FWCLEARHLP": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWMSGRUN": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := getArgString(args, 0, "")
			fmt.Printf("[MSGRUN] %s\n", msg)
			return advplrt.Nil, nil
		},
		"FWMONITORMSG": func(args []advplrt.Value) (advplrt.Value, error) {
			msg := getArgString(args, 0, "")
			fmt.Printf("[MONITOR] %s\n", msg)
			return advplrt.Nil, nil
		},
		"AMIONRESTENV": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(false), nil
		},
		"AMIIIN": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(false), nil
		},
		"CANUSEWEBUI": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"MPISSMART": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(false), nil
		},
		"MPUSERHASACCESS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"MPCRIANUMS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("000001"), nil
		},
		"MPDOCPATH": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("./"), nil
		},
		"MPDOCVIEW": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"MPBINVIEW": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"MPEXPCHK": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"MSDOCUMENT": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"MSMULTDIR": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewArray([]advplrt.Value{}), nil
		},
		"CHANGEQUERY": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"CHKADVPLSYNTAX": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"FILLGETDADOS": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWEXECLOCALIZ": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWEXISTLOCALIZ": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(false), nil
		},
		"FWQTTOCHR": func(args []advplrt.Value) (advplrt.Value, error) {
			qt := advplrt.ToString(getArg(args, 0))
			return advplrt.NewString(qt), nil
		},
		"FWREBUILDINDEX": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"FWRULESDB": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"FWGRPPRIVDB": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"FWSCHDAVAILABLE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(false), nil
		},
		"FWSCHDBYFUNCTION": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewArray([]advplrt.Value{}), nil
		},
		"FWSCHDEMPFIL": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewArray([]advplrt.Value{}), nil
		},
		"FWPDCANUSE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"FWPDLOGUSER": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWPUTSX5": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"FWX2CHAVE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},
		"FWX2UNICO": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},
		"FWX3TITULO": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},
		"FWUSREMP": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString("01"), nil
		},
		"FWVLDVINC": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"PESQBRW": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"MARKBROW": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"MAKESQLEXPR": func(args []advplrt.Value) (advplrt.Value, error) {
			expr := advplrt.ToString(getArg(args, 0))
			return advplrt.NewString(expr), nil
		},
		"MAYIUSECODE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewBool(true), nil
		},
		"RESTINTER": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"SAVEINTER": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"PUTSX1HELP": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"OLE_CREATELINK": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"PROCESSA": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"MENUDEF": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.Nil, nil
		},
		"I18N": func(args []advplrt.Value) (advplrt.Value, error) {
			key := advplrt.ToString(getArg(args, 0))
			return advplrt.NewString(key), nil
		},
		"WSADVVALUE": func(args []advplrt.Value) (advplrt.Value, error) {
			return advplrt.NewString(""), nil
		},
	}

	for name, fn := range natives {
		v.natives[name] = &advplrt.FunctionValue{Name: name, Fn: fn}
	}
}

// Helper functions

func makeArray(dims []advplrt.Value) advplrt.Value {
	if len(dims) == 0 {
		return advplrt.NewArray([]advplrt.Value{})
	}
	n, ok := dims[0].(*advplrt.NumberValue)
	if !ok || n.Val <= 0 {
		return advplrt.NewArray([]advplrt.Value{})
	}
	elems := make([]advplrt.Value, int(n.Val))
	for i := range elems {
		if len(dims) > 1 {
			elems[i] = makeArray(dims[1:])
		} else {
			elems[i] = advplrt.Nil
		}
	}
	return advplrt.NewArray(elems)
}

func getArg(args []advplrt.Value, idx int) advplrt.Value {
	if idx < len(args) {
		return args[idx]
	}
	return advplrt.Nil
}

func getArgString(args []advplrt.Value, idx int, def string) string {
	if idx < len(args) {
		return advplrt.ToString(args[idx])
	}
	return def
}

func buildOutputString(args []advplrt.Value) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = advplrt.ToString(arg)
	}
	return strings.Join(parts, " ")
}

func applyTransform(val advplrt.Value, mask string) string {
	if mask == "" {
		return advplrt.ToString(val)
	}
	if mask == "@E" {
		return advplrt.ToString(val)
	}
	if strings.Contains(mask, "9") || strings.Contains(mask, "#") {
		num := advplrt.ToFloat(val)
		decimals := 0
		if dotIdx := strings.Index(mask, "."); dotIdx >= 0 {
			decimals = len(mask) - dotIdx - 1
		}
		return strconv.FormatFloat(num, 'f', decimals, 64)
	}
	return advplrt.ToString(val)
}

func sortValues(elems []advplrt.Value) {
	for i := 1; i < len(elems); i++ {
		for j := i; j > 0; j-- {
			if advplrt.ToFloat(elems[j]) < advplrt.ToFloat(elems[j-1]) {
				elems[j], elems[j-1] = elems[j-1], elems[j]
			} else {
				break
			}
		}
	}
}

func getEnvOrDefault(name, def string) string {
	return def
}
