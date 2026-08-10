package vm

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerInterfacehttpNatives registra a família de funções HTTP*/HTTPS* da
// API TOTVS legada (Interface-HTTP). Diferente das FWHTTP* (httpclient_native.go),
// estas não têm prefixo FW e usam a convenção antiga de retorno de string do
// documento + header de resposta por referência.
func (v *VM) registerInterfacehttpNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// HTTPGet(cUrl, [cGetParms], [nTimeOut], [aHeadStr], [@cHeaderGet], [lFollowRedirect]) -> cRet
	natives["HTTPGET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.legacyHTTPRequest(legacyReqOpts{
			method:  "GET",
			args:    args,
			url:     0, get: 1, timeout: 2, head: 3, follow: 5,
		})
	}

	// HTTPPost(cUrl, [cGetParms], [cPostParms], [nTimeOut], [aHeadStr], [@cHeaderGet]) -> cRet
	natives["HTTPPOST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.legacyHTTPRequest(legacyReqOpts{
			method:  "POST",
			args:    args,
			url:     0, get: 1, post: 2, timeout: 3, head: 4,
		})
	}

	// HTTPCGet(cUrl, [cGetParms], [nTimeOut], [aHeadStr], [@cHeaderGet]) -> cRet
	natives["HTTPCGET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.legacyHTTPRequest(legacyReqOpts{
			method:  "GET",
			args:    args,
			url:     0, get: 1, timeout: 2, head: 3,
		})
	}

	// HTTPCPost(cUrl, cPostParms, [nTimeOut], [aHeadStr], [@cHeaderGet]) -> cRet
	natives["HTTPCPOST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.legacyHTTPRequest(legacyReqOpts{
			method:  "POST",
			args:    args,
			url:     0, post: 1, timeout: 2, head: 3,
		})
	}

	// HTTPQuote(cUrl, cMethod, [cGETParms], [cPOSTParms], [nTimeOut], [aHeadStr], [@cHeaderRet]) -> cRet
	natives["HTTPQUOTE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		o := legacyReqOpts{method: "GET", args: args, url: 0, get: 2, post: 3, timeout: 4, head: 5}
		if m := getArgString(args, 1, ""); m != "" {
			o.method = strings.ToUpper(m)
		}
		return v.legacyHTTPRequest(o)
	}

	// HTTPGetStatus([@cError], [lClient]) -> nRet
	// Retorna o status da última conexão HTTP/HTTPS realizada. Valores abaixo
	// de 100 representam erro. @cError (por referência) não é gravável na VM —
	// a descrição do erro fica disponível em v.legacyHTTPError (limitação
	// arquitetural documentada em docs/tdn-known-limitations.md).
	natives["HTTPGETSTATUS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(float64(v.legacyHTTPStatus)), nil
	}

	// HTTPSetPass(cUser, cPass, [lClient]) -> Nil
	natives["HTTPSETPASS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		v.legacyHTTPUser = getArgString(args, 0, "")
		v.legacyHTTPPass = getArgString(args, 1, "")
		return advplrt.Nil, nil
	}

	// SetProxy(cServer, nPort, [cUser], [cPass], [lClient]) -> Nil
	natives["SETPROXY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		v.legacyProxyServer = getArgString(args, 0, "")
		v.legacyProxyPort = int(advplrt.ToFloat(getArg(args, 1)))
		v.legacyProxyUser = getArgString(args, 2, "")
		v.legacyProxyPass = getArgString(args, 3, "")
		return advplrt.Nil, nil
	}

	// SetNoProxyFor(cDomainList, [lClient]) -> Nil
	natives["SETNOPROXYFOR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		list := getArgString(args, 0, "")
		v.legacyNoProxyFor = nil
		for _, d := range strings.Split(list, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				v.legacyNoProxyFor = append(v.legacyNoProxyFor, d)
			}
		}
		return advplrt.Nil, nil
	}

	// HTTPSGet(cURL, cCertificate, cPrivKey, cPassword, [cGETParms], [nTimeOut], [aHeadStr], [@cHeaderRet], [lClient]) -> cRet
	natives["HTTPSGET"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.legacyHTTPRequest(legacyReqOpts{
			method: "GET", args: args, isHTTPS: true,
			url: 0, cert: 1, key: 2, pass: 3, get: 4, timeout: 5, head: 6,
		})
	}

	// HTTPSPost(cURL, cCertificate, cPrivKey, cPassword, [cGETParms], [cPOSTParms], [nTimeOut], [aHeadStr], [@cHeaderRet], [lClient]) -> cRet
	natives["HTTPSPOST"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return v.legacyHTTPRequest(legacyReqOpts{
			method: "POST", args: args, isHTTPS: true,
			url: 0, cert: 1, key: 2, pass: 3, get: 4, post: 5, timeout: 6, head: 7,
		})
	}

	// HTTPSQuote(cURL, cCertificate, cPrivKey, cPassword, cMethod, [cGETParms], [cPOSTParms], [nTimeOut], [aHeadStr], [@cHeaderRet], [lClient]) -> cRet
	natives["HTTPSQUOTE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		o := legacyReqOpts{method: "GET", args: args, isHTTPS: true,
			url: 0, cert: 1, key: 2, pass: 3, get: 5, post: 6, timeout: 7, head: 8}
		if m := getArgString(args, 4, ""); m != "" {
			o.method = strings.ToUpper(m)
		}
		return v.legacyHTTPRequest(o)
	}
}

// legacyReqOpts descreve o layout posicional dos argumentos de uma chamada
// HTTP*/HTTPS* legada. Índices com valor negativo significam "parâmetro não
// presente no layout". isHTTPS indica que o certificado/chave/senha (índices
// cert/key/pass) devem ser carregados e o TLS configurado.
type legacyReqOpts struct {
	method   string
	args     []advplrt.Value
	isHTTPS  bool
	url      int
	cert     int
	key      int
	pass     int
	get      int
	post     int
	timeout  int
	head     int
	follow   int
}

// legacyHTTPRequest executa uma requisição HTTP/HTTPS da família TDN antiga.
//
// Retorna a string do documento solicitado; Nil em caso de time-out, falha de
// resolução de DNS ou erro de sintaxe na URL (per TDN). Status e header da
// resposta ficam gravados em v.legacyHTTPStatus/v.legacyHTTPHeader para
// HTTPGetStatus/@cHeaderGet (by-ref não gravável na VM).
func (v *VM) legacyHTTPRequest(o legacyReqOpts) (advplrt.Value, error) {
	v.legacyHTTPStatus = 0
	v.legacyHTTPHeader = ""
	v.legacyHTTPError = ""

	baseURL := getArgString(o.args, o.url, "")
	if baseURL == "" {
		v.legacyHTTPError = "URL is required"
		return advplrt.Nil, nil
	}

	body := ""
	if o.method != "GET" && o.method != "HEAD" && o.post >= 0 {
		body = getArgString(o.args, o.post, "")
	}

	getParms := ""
	if o.get >= 0 {
		getParms = getArgString(o.args, o.get, "")
	}

	reqURL := baseURL
	if getParms != "" {
		if strings.Contains(reqURL, "?") {
			reqURL += "&" + getParms
		} else {
			reqURL += "?" + getParms
		}
	}

	// nTimeOut em segundos; default 120 (2 minutos) per TDN.
	timeoutSec := 120
	if o.timeout >= 0 {
		if n := advplrt.ToFloat(getArg(o.args, o.timeout)); n > 0 {
			timeoutSec = int(n)
			if timeoutSec < 1 {
				timeoutSec = 1
			}
		}
	}

	headers := parseLegacyHeaders(getArg(o.args, o.head))

	// lFollowRedirect (somente HTTPGet): quando explicitamente .F., não segue
	// redirecionamentos 301/302 (retorna o header completo, incl. Location).
	followRedirect := true
	if o.follow >= 0 && !isNilValue(getArg(o.args, o.follow)) && !advplrt.ToBool(getArg(o.args, o.follow)) {
		followRedirect = false
	}

	tlsConf := &tls.Config{MinVersion: tls.VersionTLS12}
	if o.isHTTPS {
		if err := v.loadLegacyClientCert(o, tlsConf); err != nil {
			v.legacyHTTPError = err.Error()
			return advplrt.Nil, nil
		}
	}
	if os.Getenv("ADVPP_HTTP_INSECURE") != "" {
		tlsConf.InsecureSkipVerify = true
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConf,
		Proxy:           http.ProxyFromEnvironment,
	}
	if v.legacyProxyServer != "" {
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			if v.hostIsNoProxy(req.URL.Host) {
				return nil, nil
			}
			host := v.legacyProxyServer
			if v.legacyProxyPort > 0 && !strings.Contains(host, ":") {
				host = host + ":" + strconv.Itoa(v.legacyProxyPort)
			}
			if !strings.Contains(host, "://") {
				host = "http://" + host
			}
			proxyURL := &url.URL{Scheme: "http", Host: host}
			if u, err := url.Parse(host); err == nil {
				proxyURL = u
			}
			if v.legacyProxyUser != "" {
				proxyURL.User = url.UserPassword(v.legacyProxyUser, v.legacyProxyPass)
			}
			return proxyURL, nil
		}
	}

	client := &http.Client{
		Timeout:   time.Duration(timeoutSec) * time.Second,
		Transport: transport,
	}
	if !followRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest(o.method, reqURL, strings.NewReader(body))
	} else {
		req, err = http.NewRequest(o.method, reqURL, nil)
	}
	if err != nil {
		v.legacyHTTPError = fmt.Sprintf("invalid URL: %v", err)
		return advplrt.Nil, nil
	}

	for k, val := range headers {
		req.Header.Set(k, val)
	}
	if v.legacyHTTPUser != "" || v.legacyHTTPPass != "" {
		req.SetBasicAuth(v.legacyHTTPUser, v.legacyHTTPPass)
	}

	resp, err := client.Do(req)
	if err != nil {
		v.legacyHTTPError = err.Error()
		return advplrt.Nil, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		v.legacyHTTPError = err.Error()
		return advplrt.Nil, nil
	}

	v.legacyHTTPStatus = resp.StatusCode
	v.legacyHTTPHeader = buildLegacyHeaderString(resp)
	return advplrt.NewString(string(respBody)), nil
}

// loadLegacyClientCert carrega certificado + chave privada em PEM para as
// variantes HTTPS*. Paths Windows-style ("\certs\cert.pem") e arquivos
// inexistentes retornam erro (espelha a exigência TDN de path de servidor).
func (v *VM) loadLegacyClientCert(o legacyReqOpts, tlsConf *tls.Config) error {
	certPath := getArgString(o.args, o.cert, "")
	keyPath := getArgString(o.args, o.key, "")
	password := getArgString(o.args, o.pass, "")
	if certPath == "" || keyPath == "" {
		return nil
	}
	if isWindowsPath(certPath) || isWindowsPath(keyPath) {
		return fmt.Errorf("only server path are allowed to Certificate on HTTPS request")
	}
	certPEM, err := os.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return fmt.Errorf("failed to read certificate: %v", err)
	}
	keyPEM, err := os.ReadFile(filepath.Clean(keyPath))
	if err != nil {
		return fmt.Errorf("failed to read private key: %v", err)
	}
	if password != "" {
		block := pemDecode(keyPEM)
		if block != nil && x509IsEncrypted(block) {
			keyPEM = pemEncryptLegacy(block, password)
		}
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("failed to load key pair: %v", err)
	}
	tlsConf.Certificates = []tls.Certificate{cert}
	return nil
}

// parseLegacyHeaders converte o array aHeadStr em map de headers. Cada item
// pode usar "|" (ex.: "Content-Type| application/json") ou ":" (ex.:
// "User-Agent: Mozilla/4.0") como separador entre nome e valor.
func parseLegacyHeaders(v advplrt.Value) map[string]string {
	out := map[string]string{}
	arr, ok := v.(*advplrt.ArrayValue)
	if !ok {
		return out
	}
	for _, item := range arr.Elements {
		s := advplrt.ToString(item)
		sep := "|"
		idx := strings.Index(s, sep)
		if idx < 0 {
			sep = ":"
			idx = strings.Index(s, sep)
		}
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(s[:idx])
		val := strings.TrimSpace(s[idx+1:])
		if name != "" {
			out[name] = val
		}
	}
	return out
}

// buildLegacyHeaderString serializa o header de resposta no formato texto
// "Nome: Valor" por linha, para o retorno por referência @cHeaderGet/@cHeaderRet.
func buildLegacyHeaderString(resp *http.Response) string {
	var sb strings.Builder
	for name, vals := range resp.Header {
		for _, val := range vals {
			sb.WriteString(name)
			sb.WriteString(": ")
			sb.WriteString(val)
			sb.WriteString("\r\n")
		}
	}
	return sb.String()
}

// hostIsNoProxy verifica se o host da requisição está na lista de exceções de
// proxy (SetNoProxyFor), com suporte a curingas "*".
func (v *VM) hostIsNoProxy(host string) bool {
	h := strings.ToLower(host)
	for _, pattern := range v.legacyNoProxyFor {
		p := strings.ToLower(strings.TrimSpace(pattern))
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "*.") {
			suffix := strings.TrimPrefix(p, "*")
			if strings.HasSuffix(h, suffix) {
				return true
			}
		} else if strings.HasSuffix(p, ".*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(h, prefix) {
				return true
			}
		} else if h == p {
			return true
		}
	}
	return false
}

func isNilValue(v advplrt.Value) bool {
	return v == nil || v == advplrt.Nil
}

// isWindowsPath detecta paths estilo Windows ("C:\..." ou "\certs\...").
func isWindowsPath(p string) bool {
	if len(p) >= 2 && p[0] == '\\' {
		return true
	}
	return len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/')
}

// pemDecode decodifica o primeiro bloco PEM da chave privada.
func pemDecode(data []byte) *pem.Block {
	block, _ := pem.Decode(data)
	return block
}

// x509IsEncrypted verifica se um bloco PEM de chave é criptografado
// (encrypted private key PEM legacy, tipo "ENCRYPTED PRIVATE KEY" ou
// "RSA PRIVATE KEY" com headers DEK-Info).
func x509IsEncrypted(block *pem.Block) bool {
	if block == nil {
		return false
	}
	if block.Type == "ENCRYPTED PRIVATE KEY" {
		return true
	}
	_, ok := block.Headers["DEK-Info"]
	return ok
}

// pemEncryptLegacy descriptografa um bloco PEM de chave privada no formato
// legado (RFC 1423, usado pelo modelo Apache) usando a senha informada.
// Retorna o PEM com a chave já decriptografada (novo formato PKCS8) para
// alimentar tls.X509KeyPair. Se a senha não abrir o bloco, retorna o PEM
// original — o erro de decriptografia aparecerá no X509KeyPair seguinte.
func pemEncryptLegacy(block *pem.Block, password string) []byte {
	der := block.Bytes
	if x509.IsEncryptedPEMBlock(block) {
		dec, err := x509.DecryptPEMBlock(block, []byte(password))
		if err == nil {
			der = dec
		}
	}
	key, err := x509.ParsePKCS1PrivateKey(der)
	if err != nil {
		return pem.EncodeToMemory(block)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return pem.EncodeToMemory(block)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}
