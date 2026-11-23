package webui

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// poui_shim.js — ponte JS↔AdvPL injetada no index.html do app (embed).
//
//go:embed poui_shim.js
var pouiShimJS []byte

// PouiCtx é o contexto de sessão injetado no app PO-UI pelo shim
// (poui_shim.js): as chaves de sessionStorage que a
// @totvs/protheus-lib-core espera (seção 3.2 do plano da onda 2).
type PouiCtx struct {
	App, Empresa, Filial, Modulo, Usuario, Idioma, RestPrefix, Origem string
	Token                                                             string // ERPTOKEN: 32 bytes hex de crypto/rand, por sessão do FwCallApp
}

// NovoPouiCtx monta o contexto e gera o ERPTOKEN aleatório.
func NovoPouiCtx(app, empresa, filial, modulo, usuario, idioma, restPrefix, origem string) (*PouiCtx, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("ERPTOKEN: %v", err)
	}
	return &PouiCtx{
		App: app, Empresa: empresa, Filial: filial, Modulo: modulo,
		Usuario: usuario, Idioma: idioma, RestPrefix: restPrefix, Origem: origem,
		Token: hex.EncodeToString(buf),
	}, nil
}

// SessionStorage devolve as chaves e formatos exatos que a lib lê (plano
// seção 3.2 item 4). ERPAPPCONFIG/ProAppConfig apontam api_baseUrl para o
// REST montado na mesma origem.
func (c *PouiCtx) SessionStorage() map[string]string {
	appConfig, _ := json.Marshal(map[string]string{
		"name":           c.App,
		"productLine":    "protheus",
		"api_baseUrl":    c.Origem + c.RestPrefix,
		"serverBackend":  c.Origem,
		"restEntryPoint": c.RestPrefix,
	})
	proAppConfig, _ := json.Marshal(map[string]string{
		"productLine":    "protheus",
		"api_baseUrl":    c.Origem + c.RestPrefix,
		"serverBackend":  c.Origem,
		"restEntryPoint": c.RestPrefix,
	})
	proCompany, _ := json.Marshal(map[string]string{
		"Code": c.Empresa, "InternalId": c.Empresa, "CorporateName": c.Empresa,
	})
	proBranch, _ := json.Marshal(map[string]string{
		"Code": c.Filial, "InternalId": c.Empresa + c.Filial, "Description": c.Filial,
	})
	idioma := c.Idioma
	if idioma == "" {
		idioma = "pt-br"
	}
	return map[string]string{
		"ProCompany":      string(proCompany),
		"ProBranch":       string(proBranch),
		"ProAppConfig":    string(proAppConfig),
		"ERPAPPCONFIG":    string(appConfig),
		"ProModule":       c.Modulo,
		"appName":         c.App,
		"ProSystemModule": c.Modulo,
		"ProDatabase":     "",
		"ProIdiom":        idioma,
		"ProRole":         "",
		"ProUser":         c.Usuario,
		"RemoteType":      "1",
		"SocketPort":      "0",
		"ERPTOKEN":        c.Token,
	}
}

// InjetaShim insere o <script> do poui_shim.js antes do primeiro
// <script> do index.html; sem <script>, antes de </body>.
func InjetaShim(index []byte) []byte {
	shim := []byte("<script>" + string(pouiShimJS) + "</script>")
	s := string(index)
	if i := strings.Index(s, "<script"); i >= 0 {
		return []byte(s[:i] + string(shim) + s[i:])
	}
	if i := strings.Index(strings.ToLower(s), "</body>"); i >= 0 {
		return []byte(s[:i] + string(shim) + s[i:])
	}
	return append(append([]byte{}, index...), shim...)
}
