package vm

import (
	"fmt"
	"net/smtp"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// mailState é o estado Go da classe TMailMessage: parâmetros da mensagem e
// do servidor SMTP, setados via métodos antes de Send() (mesmo estilo de
// configuração por método já usado em WSRestServer/MCPServer neste
// projeto, em vez de emular a API por-propriedade do TMailMessage real do
// Protheus). Envio via net/smtp da stdlib — sem CGO, sem dependência
// externa, mesmo padrão do resto do AdvPP.
type mailState struct {
	from    string
	to      []string
	subject string
	body    string
	server  string
	port    string
	user    string
	pass    string
}

func newTMailMessageObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TMailMessage", nil)
	obj.Native = &mailState{port: "587"}
	return obj
}

// callTMailMessageMethod implementa a classe nativa TMailMessage: envio de
// e-mail real via SMTP (net/smtp, stdlib) — capacidade de compilador nova,
// motivada pela necessidade futura de mala direta do GesCon (o GesCon em
// si não consome esta classe ainda; fica disponível pro Plano 2).
func (v *VM) callTMailMessageMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*mailState)
	if !ok {
		return fmt.Errorf("TMailMessage: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		v.push(obj)
	case "SETSERVER":
		st.server = advplrt.ToString(getArg(args, 0))
		if len(args) > 1 {
			st.port = advplrt.ToString(args[1])
		}
		v.push(advplrt.Nil)
	case "SETAUTH":
		st.user = advplrt.ToString(getArg(args, 0))
		st.pass = advplrt.ToString(getArg(args, 1))
		v.push(advplrt.Nil)
	case "SETFROM":
		st.from = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
	case "ADDTO":
		st.to = append(st.to, advplrt.ToString(getArg(args, 0)))
		v.push(advplrt.Nil)
	case "SETSUBJECT":
		st.subject = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
	case "SETBODY":
		st.body = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)
	case "SEND":
		if st.server == "" {
			return advplrt.NewError("TMailMessage:Send: chame SetServer() primeiro")
		}
		if st.from == "" || len(st.to) == 0 {
			return advplrt.NewError("TMailMessage:Send: chame SetFrom() e AddTo() primeiro")
		}
		msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n",
			st.from, strings.Join(st.to, ", "), st.subject, st.body)
		var auth smtp.Auth
		if st.user != "" {
			auth = smtp.PlainAuth("", st.user, st.pass, st.server)
		}
		addr := st.server + ":" + st.port
		if err := smtp.SendMail(addr, auth, st.from, st.to, []byte(msg)); err != nil {
			v.push(advplrt.False)
			return advplrt.NewError(fmt.Sprintf("TMailMessage:Send: %v", err))
		}
		v.push(advplrt.True)
	default:
		return fmt.Errorf("TMailMessage: método desconhecido %s", method)
	}
	return nil
}
