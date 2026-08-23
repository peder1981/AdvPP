package vm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// smartlinkState é o estado Go da classe FwTotvsLinkClient (TDN:
// Frameworks / FWCarolWizard / FwTotvsLinkClient).
//
// A TDN documenta a API pública (New/Receive/GetMessage/Success/Fail/Send/
// SendAudience/GetError/Set-GetRefreshToken/Destroy) e o transporte real
// ("Smart Link... utiliza um serviço de fila (rabbit) recebendo
// requisições através do protocolo HTTP"), mas não publica: a URL do
// FwTotvsAppsRegistry que resolve o endpoint real do Smart Link, o path
// exato de cada operação REST, nem o formato exato do objeto de mensagem
// devolvido por GetMessage() ("self:oMessage", tipo não detalhado).
//
// Diferente do tGrpc (onde gRPC Server Reflection permite descobrir o
// schema real em runtime, sem precisar de configuração externa), aqui não
// existe mecanismo de descoberta: por isso o endpoint, o token OAuth2 e o
// tenant são lidos de variáveis de ambiente configuráveis (mesmo padrão já
// usado por ADVPP_SFTP_KNOWN_HOSTS/ADVPP_HTTP_INSECURE) — sem elas
// configuradas, os métodos fazem uma tentativa HTTP real contra "" e
// falham honestamente (erro de rede real, nunca simulado). Os paths REST
// (/message, /message/ack, /message/nack) e o grant OAuth2
// client_credentials (RFC 6749 §4.4, padrão público — não específico do
// Smart Link) são 🟡 INFERIDO: convenção razoável, não confirmada contra
// o serviço real. Ajustável por quem tiver acesso à doc interna do
// FwTotvsAppsRegistry.
type smartlinkState struct {
	refreshToken bool

	baseURL      string
	tokenURL     string
	clientID     string
	clientSecret string
	tenantID     string

	accessToken string
	tokenExpiry time.Time

	oMessage advplrt.Value // último objeto de mensagem recebido (Receive/GetMessage)
	lastErr  string

	client *http.Client
}

func newSmartlinkState() *smartlinkState {
	return &smartlinkState{
		baseURL:      strings.TrimRight(os.Getenv("ADVPP_SMARTLINK_BASEURL"), "/"),
		tokenURL:     os.Getenv("ADVPP_SMARTLINK_TOKENURL"),
		clientID:     os.Getenv("ADVPP_SMARTLINK_CLIENTID"),
		clientSecret: os.Getenv("ADVPP_SMARTLINK_CLIENTSECRET"),
		tenantID:     os.Getenv("ADVPP_SMARTLINK_TENANTID"),
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

func newSmartlinkObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("FWTOTVSLINKCLIENT", nil)
	obj.Native = newSmartlinkState()
	return obj
}

// ensureToken obtém (ou reaproveita, se ainda válido e sem refresh
// forçado) um access token real via OAuth2 "client_credentials" (RFC
// 6749 §4.4 — grant padrão público, não específico do Smart Link) contra
// st.tokenURL. Sem tokenURL/clientID/clientSecret configurados, devolve
// erro real explicando a configuração ausente (não simula sucesso).
func (st *smartlinkState) ensureToken() error {
	if st.tokenURL == "" || st.clientID == "" || st.clientSecret == "" {
		return fmt.Errorf("ADVPP_SMARTLINK_TOKENURL/CLIENTID/CLIENTSECRET não configurados")
	}
	if !st.refreshToken && st.accessToken != "" && time.Now().Before(st.tokenExpiry) {
		return nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", st.clientID)
	form.Set("client_secret", st.clientSecret)

	req, err := http.NewRequest(http.MethodPost, st.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := st.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("token endpoint retornou HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return fmt.Errorf("resposta do token endpoint não é JSON OAuth2 válido: %w", err)
	}
	if tok.AccessToken == "" {
		return fmt.Errorf("resposta do token endpoint sem access_token")
	}
	st.accessToken = tok.AccessToken
	expires := tok.ExpiresIn
	if expires <= 0 {
		expires = 3600
	}
	st.tokenExpiry = time.Now().Add(time.Duration(expires) * time.Second)
	return nil
}

// smartlinkDo monta e executa uma requisição HTTP real contra
// {baseURL}{path}, com Authorization Bearer (quando ensureToken tiver
// sucesso) e headers extras opcionais.
func (st *smartlinkState) smartlinkDo(method, path, body string, extraHeaders map[string]string) (*http.Response, string, error) {
	if st.baseURL == "" {
		return nil, "", fmt.Errorf("ADVPP_SMARTLINK_BASEURL não configurado")
	}
	if err := st.ensureToken(); err != nil {
		// Token é opcional: alguns ambientes de teste do Smart Link não
		// exigem OAuth. Registra o erro mas só aborta se não houver
		// tokenURL configurado nenhuma vez (config claramente incompleta
		// para o fluxo real) — aqui apenas prossegue sem Authorization.
		st.lastErr = err.Error()
	}

	req, err := http.NewRequest(method, st.baseURL+path, strings.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	if st.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
	}
	if st.tenantID != "" {
		req.Header.Set("X-Tenant-Id", st.tenantID)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := st.client.Do(req)
	if err != nil {
		st.lastErr = err.Error()
		return nil, "", err
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(respBody), nil
}

func (v *VM) callFwTotvsLinkClientMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*smartlinkState)
	if !ok {
		return fmt.Errorf("FwTotvsLinkClient: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		// New( [ lRefreshToken ] ) -> Nil
		if len(args) > 0 && args[0] != nil && args[0] != advplrt.Nil {
			if b, ok := args[0].(*advplrt.BoolValue); ok {
				st.refreshToken = b.Val
			}
		}
		if st.refreshToken {
			if err := st.ensureToken(); err != nil {
				st.lastErr = err.Error()
			}
		}
		v.push(obj)
		return nil

	case "SETREFRESHTOKEN":
		if b, ok := getArg(args, 0).(*advplrt.BoolValue); ok {
			st.refreshToken = b.Val
		}
		v.push(advplrt.Nil)
		return nil

	case "GETREFRESHTOKEN":
		v.push(advplrt.NewBool(st.refreshToken))
		return nil

	case "GETERROR":
		v.push(advplrt.NewString(st.lastErr))
		return nil

	case "GETTENANTCLIENT":
		// 🟡 INFERIDO: não documentado na página TDN de FwTotvsLinkClient
		// (só encontrado em uso real: backoffice.techfin.util.smartlink.tlpp,
		// rh.sigagpe.insights.sendendcalc.tlpp). Sem saber como a classe
		// real deriva o tenant (provavelmente de um claim do JWT do
		// access_token, não documentado), devolve o valor configurado
		// explicitamente via ADVPP_SMARTLINK_TENANTID.
		v.push(advplrt.NewString(st.tenantID))
		return nil

	case "SEND":
		// Send( < cType >, < cMessage > ) -> lSuccess
		cType := advplrt.ToString(getArg(args, 0))
		cMessage := advplrt.ToString(getArg(args, 1))
		resp, _, err := st.smartlinkDo(http.MethodPost, "/message", cMessage, map[string]string{"X-Message-Type": cType})
		if err != nil {
			v.push(advplrt.False)
			return nil
		}
		ok := resp.StatusCode >= 200 && resp.StatusCode < 300
		if !ok {
			st.lastErr = fmt.Sprintf("Send: HTTP %d", resp.StatusCode)
		}
		v.push(advplrt.NewBool(ok))
		return nil

	case "SENDAUDIENCE":
		// SendAudience( < cType >, < cAudience >, < cMessage >, [ aHeader ] ) -> lSuccess
		cType := advplrt.ToString(getArg(args, 0))
		cAudience := advplrt.ToString(getArg(args, 1))
		cMessage := advplrt.ToString(getArg(args, 2))
		headers := map[string]string{"X-Message-Type": cType, "X-Audience": cAudience}
		for k, val := range parseLegacyHeaders(getArg(args, 3)) {
			headers[k] = val
		}
		resp, _, err := st.smartlinkDo(http.MethodPost, "/message", cMessage, headers)
		if err != nil {
			v.push(advplrt.False)
			return nil
		}
		ok := resp.StatusCode >= 200 && resp.StatusCode < 300
		if !ok {
			st.lastErr = fmt.Sprintf("SendAudience: HTTP %d", resp.StatusCode)
		}
		v.push(advplrt.NewBool(ok))
		return nil

	case "RECEIVE":
		// Receive() -> lSuccess
		resp, body, err := st.smartlinkDo(http.MethodGet, "/message", "", nil)
		if err != nil {
			v.push(advplrt.False)
			return nil
		}
		if resp.StatusCode == 204 || resp.StatusCode == 404 {
			st.oMessage = advplrt.Nil
			v.push(advplrt.False)
			return nil
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			st.lastErr = fmt.Sprintf("Receive: HTTP %d", resp.StatusCode)
			v.push(advplrt.False)
			return nil
		}
		// self:oMessage: tipo exato não documentado pela TDN. Melhor
		// esforço real (não simulado): se o corpo é JSON válido, decodifica
		// para um objeto JsonObject de verdade (mesma máquina usada por
		// oJson:fromJson em todo o resto do VM); senão devolve um
		// JsonObject com o corpo bruto em "body".
		var parsed interface{}
		if json.Unmarshal([]byte(body), &parsed) == nil {
			st.oMessage = jsonToAdvplValue(parsed)
		} else {
			wrapper := advplrt.NewObject("JsonObject", nil)
			wrapper.SetProp("BODY", advplrt.NewString(body))
			st.oMessage = wrapper
		}
		v.push(advplrt.True)
		return nil

	case "GETMESSAGE":
		if st.oMessage == nil {
			v.push(advplrt.Nil)
			return nil
		}
		v.push(st.oMessage)
		return nil

	case "SUCCESS":
		// Success() -> lSuccess
		resp, _, err := st.smartlinkDo(http.MethodPost, "/message/ack", "", nil)
		if err != nil {
			v.push(advplrt.False)
			return nil
		}
		v.push(advplrt.NewBool(resp.StatusCode >= 200 && resp.StatusCode < 300))
		return nil

	case "FAIL":
		// Fail() -> lSuccess (envia a mensagem posicionada para a fila DLQ)
		resp, _, err := st.smartlinkDo(http.MethodPost, "/message/nack", "", nil)
		if err != nil {
			v.push(advplrt.False)
			return nil
		}
		v.push(advplrt.NewBool(resp.StatusCode >= 200 && resp.StatusCode < 300))
		return nil

	case "DESTROY":
		st.oMessage = nil
		st.accessToken = ""
		v.push(advplrt.Nil)
		return nil

	default:
		return fmt.Errorf("unknown method %s on FwTotvsLinkClient", method)
	}
}
