package vm

import (
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// ftpState é o estado Go da classe TFtpClient (TDN: Classes/Não Visual /
// TFtpClient, RFC 959). Implementação real via net/textproto (protocolo de
// controle FTP em texto) + modo passivo (PASV) para conexões de dados —
// não simulada: abre conexões TCP reais e fala o protocolo FTP real.
type ftpState struct {
	ctrl    *textproto.Conn
	host    string
	lastMsg string
	curType int // TDN nTransferType: 0 = ASCII (padrão), 1 = Binário/Imagem
	mlLines []string

	connectTimeout int
	controlPort    int
	dataPort       int
	fireWallMode   bool
}

func newFTPState() *ftpState {
	return &ftpState{connectTimeout: 30, controlPort: 21, dataPort: 20}
}

func newFTPClientObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TFTPCLIENT", nil)
	st := newFTPState()
	obj.Native = st
	obj.Props["NCONNECTTIMEOUT"] = advplrt.NewNumber(float64(st.connectTimeout))
	obj.Props["NCONTROLPORT"] = advplrt.NewNumber(float64(st.controlPort))
	obj.Props["NDATAPORT"] = advplrt.NewNumber(float64(st.dataPort))
	obj.Props["NTRANSFERMODE"] = advplrt.NewNumber(0)
	obj.Props["NTRANSFERTYPE"] = advplrt.NewNumber(0)
	obj.Props["BFIREWALLMODE"] = advplrt.False
	obj.Props["CERRORSTRING"] = advplrt.NewString("")
	return obj
}

// ftpCmd envia um comando de controle e lê a resposta (possivelmente
// multi-linha), atualizando lastMsg/mlLines. Retorna o código numérico da
// resposta (0 se não houver conexão ou erro de I/O).
func (st *ftpState) ftpCmd(format string, args ...interface{}) int {
	if st.ctrl == nil {
		st.lastMsg = "Not connected"
		return 0
	}
	_, err := st.ctrl.Cmd(format, args...)
	if err != nil {
		st.lastMsg = err.Error()
		return 0
	}
	code, msg, rerr := st.ctrl.ReadResponse(0)
	st.lastMsg = msg
	st.mlLines = strings.Split(msg, "\n")
	if rerr != nil {
		if _, ok := rerr.(*textproto.Error); !ok {
			return 0
		}
	}
	return code
}

var ftpPasvRe = regexp.MustCompile(`\((\d+),(\d+),(\d+),(\d+),(\d+),(\d+)\)`)

// ftpPassiveConn entra em modo passivo (PASV) e abre a conexão de dados
// correspondente — implementação real (não simulada) do modo de
// transferência de dados padrão do RFC 959 §4.1.2.
func (st *ftpState) ftpPassiveConn() (net.Conn, int, error) {
	if st.ctrl == nil {
		return nil, 0, fmt.Errorf("not connected")
	}
	_, err := st.ctrl.Cmd("PASV")
	if err != nil {
		return nil, 0, err
	}
	code, msg, _ := st.ctrl.ReadResponse(227)
	st.lastMsg = msg
	if code != 227 {
		return nil, code, fmt.Errorf("PASV failed: %s", msg)
	}
	m := ftpPasvRe.FindStringSubmatch(msg)
	if m == nil {
		return nil, code, fmt.Errorf("PASV: could not parse response %q", msg)
	}
	p1, _ := strconv.Atoi(m[5])
	p2, _ := strconv.Atoi(m[6])
	ip := fmt.Sprintf("%s.%s.%s.%s", m[1], m[2], m[3], m[4])
	port := p1*256 + p2
	conn, derr := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), time.Duration(st.connectTimeout)*time.Second)
	if derr != nil {
		return nil, code, derr
	}
	return conn, code, nil
}

func (v *VM) callTFtpClientMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*ftpState)
	if !ok {
		return fmt.Errorf("TFtpClient: objeto sem estado interno")
	}
	syncProps := func() {
		obj.Props["CERRORSTRING"] = advplrt.NewString(st.lastMsg)
		obj.Props["NTRANSFERTYPE"] = advplrt.NewNumber(float64(st.curType))
	}

	switch method {
	case "NEW":
		v.push(obj)
		return nil

	case "FTPCONNECT":
		// FTPConnect( < cHost >, [ nPort ], [ cUser ], [ cPassword ] ) -> nRet
		// NOTA DE CONFIANÇA (🟡 INFERIDO): a página TDN fornecida mostra o
		// exemplo com um único argumento (host) e não publica a subpágina
		// de sintaxe detalhada de FTPConnect neste PDF. nPort/cUser/
		// cPassword são inferidos por analogia ao uso comum de clientes
		// FTP (default port 21, usuário "anonymous"); não verificado
		// contra a página de detalhe real do método.
		host := advplrt.ToString(getArg(args, 0))
		port := st.controlPort
		if len(args) > 1 && args[1] != nil && args[1] != advplrt.Nil {
			port = int(advplrt.ToFloat(args[1]))
		}
		user := "anonymous"
		if len(args) > 2 && args[2] != nil && args[2] != advplrt.Nil {
			user = advplrt.ToString(args[2])
		}
		pass := "anonymous@"
		if len(args) > 3 && args[3] != nil && args[3] != advplrt.Nil {
			pass = advplrt.ToString(args[3])
		}
		st.host = host

		conn, derr := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Duration(st.connectTimeout)*time.Second)
		if derr != nil {
			st.lastMsg = derr.Error()
			syncProps()
			v.push(advplrt.NewNumber(-1))
			return nil
		}
		st.ctrl = textproto.NewConn(conn)
		code, msg, _ := st.ctrl.ReadResponse(220)
		st.lastMsg = msg
		if code != 220 {
			syncProps()
			v.push(advplrt.NewNumber(float64(code)))
			return nil
		}
		code = st.ftpCmd("USER %s", user)
		if code == 331 { // precisa de senha
			code = st.ftpCmd("PASS %s", pass)
		}
		syncProps()
		if code == 230 {
			v.push(advplrt.NewNumber(0))
		} else {
			v.push(advplrt.NewNumber(float64(code)))
		}
		return nil

	case "GETLASTRESPONSE":
		v.push(advplrt.NewString(st.lastMsg))
		return nil

	case "GETCURDIR":
		// GetCurDir( < @cDir > ) -> nRet
		// LIMITAÇÃO CONHECIDA (@var): @cDir é escalar e não é populado
		// neste VM (mesma limitação arquitetural documentada em
		// docs/tdn-known-limitations.md); o diretório atual real fica em
		// GetLastResponse() após a chamada (resposta bruta do comando
		// PWD).
		code := st.ftpCmd("PWD")
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "GETHELP":
		cmd := advplrt.ToString(getArg(args, 0))
		var code int
		if cmd == "" {
			code = st.ftpCmd("HELP")
		} else {
			code = st.ftpCmd("HELP %s", cmd)
		}
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "GETMLCOUNT":
		v.push(advplrt.NewNumber(float64(len(st.mlLines))))
		return nil

	case "GETMLLINE":
		idx := int(advplrt.ToFloat(getArg(args, 0)))
		if idx < 0 || idx >= len(st.mlLines) {
			v.push(advplrt.NewString(""))
			return nil
		}
		v.push(advplrt.NewString(st.mlLines[idx]))
		return nil

	case "MKDIR":
		dir := advplrt.ToString(getArg(args, 0))
		code := st.ftpCmd("MKD %s", dir)
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "RMDIR":
		dir := advplrt.ToString(getArg(args, 0))
		code := st.ftpCmd("RMD %s", dir)
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "CHDIR":
		dir := advplrt.ToString(getArg(args, 0))
		code := st.ftpCmd("CWD %s", dir)
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "CDUP":
		code := st.ftpCmd("CDUP")
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "DIRECTORY":
		// Directory( [ cMask ] ) -> nRet — lista via LIST em modo
		// passivo; as linhas retornadas ficam acessíveis via
		// GetMLCount/GetMLLine (mesmo padrão de GetHelp).
		mask := advplrt.ToString(getArg(args, 0))
		dconn, code, derr := st.ftpPassiveConn()
		if derr != nil {
			syncProps()
			v.push(advplrt.NewNumber(float64(code)))
			return nil
		}
		cmdCode := 0
		if mask == "" || mask == "*" {
			cmdCode = st.ftpCmdNoResponse("LIST")
		} else {
			cmdCode = st.ftpCmdNoResponse("LIST %s", mask)
		}
		if cmdCode == 0 {
			dconn.Close()
			syncProps()
			v.push(advplrt.NewNumber(0))
			return nil
		}
		data, _ := io.ReadAll(dconn)
		dconn.Close()
		fcode, fmsg, _ := st.ctrl.ReadResponse(226)
		st.lastMsg = fmsg
		lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
		st.mlLines = lines
		syncProps()
		v.push(advplrt.NewNumber(float64(fcode)))
		return nil

	case "SENDFILE":
		local := advplrt.ToString(getArg(args, 0))
		remote := advplrt.ToString(getArg(args, 1))
		data, rerr := os.ReadFile(local)
		if rerr != nil {
			st.lastMsg = rerr.Error()
			syncProps()
			v.push(advplrt.NewNumber(-1))
			return nil
		}
		dconn, code, derr := st.ftpPassiveConn()
		if derr != nil {
			syncProps()
			v.push(advplrt.NewNumber(float64(code)))
			return nil
		}
		cmdCode := st.ftpCmdNoResponse("STOR %s", remote)
		if cmdCode == 0 {
			dconn.Close()
			syncProps()
			v.push(advplrt.NewNumber(0))
			return nil
		}
		_, werr := dconn.Write(data)
		dconn.Close()
		if werr != nil {
			st.lastMsg = werr.Error()
			syncProps()
			v.push(advplrt.NewNumber(-1))
			return nil
		}
		fcode, fmsg, _ := st.ctrl.ReadResponse(226)
		st.lastMsg = fmsg
		syncProps()
		v.push(advplrt.NewNumber(float64(fcode)))
		return nil

	case "RECEIVEFILE":
		remote := advplrt.ToString(getArg(args, 0))
		local := advplrt.ToString(getArg(args, 1))
		dconn, code, derr := st.ftpPassiveConn()
		if derr != nil {
			syncProps()
			v.push(advplrt.NewNumber(float64(code)))
			return nil
		}
		cmdCode := st.ftpCmdNoResponse("RETR %s", remote)
		if cmdCode == 0 {
			dconn.Close()
			syncProps()
			v.push(advplrt.NewNumber(0))
			return nil
		}
		data, rerr := io.ReadAll(dconn)
		dconn.Close()
		if rerr != nil {
			st.lastMsg = rerr.Error()
			syncProps()
			v.push(advplrt.NewNumber(-1))
			return nil
		}
		if werr := os.WriteFile(local, data, 0o644); werr != nil {
			st.lastMsg = werr.Error()
			syncProps()
			v.push(advplrt.NewNumber(-1))
			return nil
		}
		fcode, fmsg, _ := st.ctrl.ReadResponse(226)
		st.lastMsg = fmsg
		syncProps()
		v.push(advplrt.NewNumber(float64(fcode)))
		return nil

	case "RENAMEFILE":
		from := advplrt.ToString(getArg(args, 0))
		to := advplrt.ToString(getArg(args, 1))
		code := st.ftpCmd("RNFR %s", from)
		if code == 350 {
			code = st.ftpCmd("RNTO %s", to)
		}
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "DELETEFILE":
		file := advplrt.ToString(getArg(args, 0))
		code := st.ftpCmd("DELE %s", file)
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "GETTYPE":
		v.push(advplrt.NewNumber(float64(st.curType)))
		return nil

	case "SETTYPE":
		nType := int(advplrt.ToFloat(getArg(args, 0)))
		typeChar := "A"
		if nType != 0 {
			typeChar = "I"
		}
		code := st.ftpCmd("TYPE %s", typeChar)
		if code == 200 {
			st.curType = nType
		}
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "NOOP":
		code := st.ftpCmd("NOOP")
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "QUOTE":
		raw := advplrt.ToString(getArg(args, 0))
		code := st.ftpCmd("%s", raw)
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	case "CLOSE":
		code := 0
		if st.ctrl != nil {
			code = st.ftpCmd("QUIT")
			st.ctrl.Close()
			st.ctrl = nil
		}
		syncProps()
		v.push(advplrt.NewNumber(float64(code)))
		return nil

	default:
		return fmt.Errorf("unknown method %s on TFtpClient", method)
	}
}

// ftpCmdNoResponse envia um comando de controle que antecede uma
// transferência de dados (LIST/STOR/RETR) e lê só a resposta preliminar
// (1xx), sem consumir a resposta final (226/550), que é lida depois de a
// conexão de dados terminar. Retorna 0 em erro de I/O.
func (st *ftpState) ftpCmdNoResponse(format string, args ...interface{}) int {
	if st.ctrl == nil {
		return 0
	}
	_, err := st.ctrl.Cmd(format, args...)
	if err != nil {
		st.lastMsg = err.Error()
		return 0
	}
	code, msg, _ := st.ctrl.ReadResponse(0)
	st.lastMsg = msg
	if code >= 100 && code < 200 {
		return code
	}
	if code >= 400 {
		return 0
	}
	return code
}
