package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/advpl/compiler/pkg/db"
	"github.com/advpl/compiler/pkg/rest"
	"github.com/advpl/compiler/pkg/tools/shared"
	"github.com/advpl/compiler/pkg/vm"
)

// serve_framework.go — endpoints de framework consumidos pela
// @totvs/protheus-lib-core dentro dos apps PO-UI (Task 2.5 do plano
// da onda 2). Registrados no mesmo rest.Server das rotas do usuário,
// sob RestPrefix, ANTES delas — fonte do usuário que declarar a mesma
// rota vence (AddRoute substitui method+path).
//
// Formatos conferidos no fesm2022 do @totvs/protheus-lib-core 21.1.4:
// listas são {"items":[...],"hasNext":bool}; ProAdapterBaseV2 adiciona
// "remainingRecords". Parâmetros ignorados estão documentados no
// COMPONENT_STATUS.md.

var frameworkIdentRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// frameworkSessão devolve os dados de sessão usados pelas respostas —
// os mesmos defaults do PouiCtx (empresa/filial "01", módulo CUSTOM,
// usuário do SO).
func frameworkSessao() (empresa, filial, modulo, usuario string) {
	usuario = os.Getenv("USER")
	if usuario == "" {
		usuario = os.Getenv("USERNAME")
	}
	if usuario == "" {
		usuario = "admin"
	}
	return "01", "01", "CUSTOM", usuario
}

func fwJSON(v any) *rest.Response {
	b, _ := json.Marshal(v)
	return &rest.Response{Status: 200, ContentType: "application/json", Body: b}
}

func fwErro(status int, msg string) *rest.Response {
	b, _ := json.Marshal(map[string]any{"code": status, "message": msg})
	return &rest.Response{Status: status, ContentType: "application/json", Body: b}
}

func fwLista(items []any) *rest.Response {
	if items == nil {
		items = []any{}
	}
	return fwJSON(map[string]any{"items": items, "hasNext": false})
}

// frameworkEngine abre uma conexão SQLite por requisição (o endpoint é
// raro e a engine não é thread-safe para uso concorrente).
func frameworkEngine(opts *Options) (*db.SQLiteEngine, error) {
	return db.NewSQLiteEngine(shared.ResolveDatabasePath(opts.dbPath))
}

// sx3Mapa devolve o mapa prop->MapaCampo das colunas do SX3 do alias —
// base do genericList/genericQuery (só campos do dicionário).
func sx3Mapa(eng *db.SQLiteEngine, alias string) (map[string]vm.MapaCampo, error) {
	rows, err := eng.QueryRows(
		"SELECT X3_CAMPO, X3_TIPO FROM SX3 WHERE UPPER(X3_ARQUIVO) = ?", alias)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("tabela %s sem SX3", alias)
	}
	mapa := make(map[string]vm.MapaCampo, len(rows))
	for _, r := range rows {
		campo := strings.TrimSpace(r["X3_CAMPO"])
		if campo == "" {
			continue
		}
		mapa[campo] = vm.MapaCampo{
			Prop: campo, Coluna: campo,
			Tipo: strings.TrimSpace(r["X3_TIPO"]), Filtra: true, Ordena: true,
		}
	}
	return mapa, nil
}

// frameworkQuery monta e executa a consulta paginada do genericList/
// genericQuery. Devolve items, hasNext e remainingRecords.
func frameworkQuery(eng *db.SQLiteEngine, opts *Options, r *rest.Request, multiAlias bool) (*rest.Response, error) {
	q := r.Query
	var aliases []string
	if multiAlias {
		for _, t := range strings.Split(q.Get("tables"), ",") {
			if t = strings.TrimSpace(t); t != "" {
				aliases = append(aliases, strings.ToUpper(t))
			}
		}
	} else {
		aliases = []string{strings.ToUpper(strings.TrimSpace(q.Get("alias")))}
	}
	if len(aliases) == 0 || aliases[0] == "" {
		return nil, fmt.Errorf("alias/tables obrigatório")
	}
	for _, a := range aliases {
		if !frameworkIdentRe.MatchString(a) {
			return nil, fmt.Errorf("alias inválido %q", a)
		}
	}
	alias := aliases[0] // SX3/validação pela primeira tabela

	mapa, err := sx3Mapa(eng, alias)
	if err != nil {
		return nil, err
	}

	page, _ := strconv.Atoi(q.Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	// WHERE: filter (OData) + where (mesmo dialeto OData) + deletedFilter.
	var whereSQL string
	var args []any
	for _, expr := range []string{q.Get("filter"), q.Get("where")} {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		frag, fargs, err := vm.TraduzFiltro(expr, mapa)
		if err != nil {
			return nil, fmt.Errorf("filtro: %v", err)
		}
		if frag != "" {
			if whereSQL != "" {
				whereSQL += " AND "
			}
			whereSQL += "(" + frag + ")"
			args = append(args, fargs...)
		}
	}
	if q.Get("deletedFilter") == "true" {
		// D_E_L_E_T_ é a coluna de exclusão lógica do Protheus; só aplica
		// se a tabela a tiver (SX3 não garante — checa no pragma).
		if temColuna(eng, alias, "D_E_L_E_T_") {
			if whereSQL != "" {
				whereSQL += " AND "
			}
			whereSQL += "(D_E_L_E_T_ IS NULL OR D_E_L_E_T_ = ' ')"
		}
	}

	// SELECT: fields explícitos validados pelo SX3, senão *.
	cols := "*"
	if f := strings.TrimSpace(q.Get("fields")); f != "" {
		lista, err := vm.TraduzCampos(f, mapa)
		if err != nil {
			return nil, fmt.Errorf("fields: %v", err)
		}
		cols = strings.Join(lista, ", ")
	}

	orderSQL := ""
	if o := strings.TrimSpace(q.Get("order")); o != "" {
		frag, err := vm.TraduzOrdem(o, mapa)
		if err != nil {
			return nil, fmt.Errorf("order: %v", err)
		}
		if frag != "" {
			orderSQL = " ORDER BY " + frag
		}
	}

	from := strings.Join(aliases, ", ")
	where := ""
	if whereSQL != "" {
		where = " WHERE " + whereSQL
	}

	// remainingRecords: COUNT com os mesmos filtros, sem paginação.
	var remaining int
	countSQL := fmt.Sprintf("SELECT COUNT(*) AS Q FROM %s%s", from, where)
	if rows, err := eng.QueryRows(countSQL, args...); err == nil && len(rows) > 0 {
		remaining, _ = strconv.Atoi(rows[0]["Q"])
	}

	offset := (page - 1) * pageSize
	dataSQL := fmt.Sprintf("SELECT %s FROM %s%s%s LIMIT ? OFFSET ?",
		cols, from, where, orderSQL)
	dataArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := eng.QueryRows(dataSQL, dataArgs...)
	if err != nil {
		return nil, err
	}

	hasNext := offset+len(rows) < remaining
	items := make([]any, 0, len(rows))
	for _, row := range rows {
		obj := map[string]any{}
		for k, val := range row {
			obj[k] = val
		}
		items = append(items, obj)
	}
	return fwJSON(map[string]any{
		"items":            items,
		"hasNext":          hasNext,
		"remainingRecords": remaining,
	}), nil
}

// temColuna verifica a existência de uma coluna física na tabela.
func temColuna(eng *db.SQLiteEngine, alias, coluna string) bool {
	rows, err := eng.QueryRows(fmt.Sprintf("SELECT name FROM pragma_table_info('%s') WHERE name = ?", alias), coluna)
	return err == nil && len(rows) > 0
}

// registraFrameworkRoutes monta os endpoints fixos da lib no servidor
// REST, antes das rotas do usuário (que podem sobrescrevê-los).
func registraFrameworkRoutes(srv *rest.Server, opts *Options) {
	empresa, filial, modulo, usuario := frameworkSessao()
	add := func(method, path string, h func(*rest.Request) *rest.Response) {
		srv.AddRoute(rest.Route{Method: method, Path: path, Raw: h})
	}
	get := func(path string, h func(*rest.Request) *rest.Response) {
		add("GET", path, h)
	}

	get("/api/framework/environment/v1/companies", func(r *rest.Request) *rest.Response {
		item := map[string]any{
			"Code": empresa, "InternalId": empresa, "CorporateName": empresa,
		}
		// SM0 local, se existir: lista as empresas cadastradas.
		if eng, err := frameworkEngine(opts); err == nil {
			defer eng.Close()
			if rows, err := eng.QueryRows("SELECT M0_CODIGO, M0_NOME FROM SM0"); err == nil && len(rows) > 0 {
				items := make([]any, 0, len(rows))
				for _, row := range rows {
					cod := strings.TrimSpace(row["M0_CODIGO"])
					items = append(items, map[string]any{
						"Code": cod, "InternalId": cod,
						"CorporateName": strings.TrimSpace(row["M0_NOME"]),
					})
				}
				return fwLista(items)
			}
		}
		return fwLista([]any{item})
	})

	get("/api/framework/environment/v1/branches", func(r *rest.Request) *rest.Response {
		return fwLista([]any{map[string]any{
			"Code": filial, "InternalId": empresa + filial,
			"Description": filial, "CompanyCode": empresa,
			"EnterpriseGroup": empresa,
		}})
	})

	currentUser := map[string]any{
		"id": usuario, "userName": usuario, "displayName": usuario,
		"complete_name": usuario, "emails": []any{}, "active": true,
	}
	get("/api/framework/current-user", func(r *rest.Request) *rest.Response {
		return fwJSON(currentUser)
	})
	get("/api/framework/v1/users/{id}", func(r *rest.Request) *rest.Response {
		u := map[string]any{}
		for k, v := range currentUser {
			u[k] = v
		}
		return fwJSON(u)
	})

	get("/api/framework/v1/systemModules", func(r *rest.Request) *rest.Response {
		return fwJSON(map[string]any{
			"items": []any{map[string]any{
				"id": modulo, "name": modulo, "description": modulo,
				"code": modulo,
			}},
			"hasNext": false,
		})
	})

	get("/api/protheus/session/v1/defaults", func(r *rest.Request) *rest.Response {
		return fwJSON(map[string]any{
			"company_code": empresa, "company_description": empresa,
			"branch_code": filial, "branch_description": filial,
			"environment_code": modulo, "environment_description": modulo,
			"environment_when": true,
		})
	})

	get("/api/framework/v1/FwRestTranslate/{x}", func(r *rest.Request) *rest.Response {
		return fwJSON(map[string]any{})
	})

	get("/api/language/v1/protheus-suported-languages", func(r *rest.Request) *rest.Response {
		return fwLista([]any{
			map[string]any{"code": "pt-br"},
			map[string]any{"code": "en-us"},
			map[string]any{"code": "es-es"},
		})
	})

	get("/api/brand/v1/erp-brand", func(r *rest.Request) *rest.Response {
		return fwJSON(map[string]any{})
	})

	// Acesso livre: a sessão local não tem controle de acesso.
	get("/api/framework/v1/accessPrivilegesServices/aliasAccess/{x}", func(r *rest.Request) *rest.Response {
		return fwJSON(true)
	})
	get("/api/framework/v1/accessPrivilegesServices/functionAccess/{x}", func(r *rest.Request) *rest.Response {
		return fwJSON(true)
	})

	// genericList (uma tabela) e genericQuery (lista de tabelas).
	get("/api/framework/v1/genericList", func(r *rest.Request) *rest.Response {
		eng, err := frameworkEngine(opts)
		if err != nil {
			return fwErro(500, err.Error())
		}
		defer eng.Close()
		resp, err := frameworkQuery(eng, opts, r, false)
		if err != nil {
			return fwErro(400, err.Error())
		}
		return resp
	})
	get("/api/framework/v1/genericQuery", func(r *rest.Request) *rest.Response {
		eng, err := frameworkEngine(opts)
		if err != nil {
			return fwErro(500, err.Error())
		}
		defer eng.Close()
		resp, err := frameworkQuery(eng, opts, r, true)
		if err != nil {
			return fwErro(400, err.Error())
		}
		return resp
	})

	// OAuth2: a sessão já é local — o token emitido é um JWT sem
	// assinatura (header.payload.) no formato que getTokenPayload() do
	// lib-core decodifica (payload.userid/exp).
	add("POST", "/api/oauth2/v1/token", func(r *rest.Request) *rest.Response {
		return fwToken()
	})
	add("GET", "/api/oauth2/v1/token", func(r *rest.Request) *rest.Response {
		return fwToken()
	})
	add("POST", "/api/oauth2/v1/batch-close-tabs", func(r *rest.Request) *rest.Response {
		return fwJSON(map[string]any{})
	})
	add("GET", "/api/oauth2/v1/batch-close-tabs", func(r *rest.Request) *rest.Response {
		return fwJSON(map[string]any{})
	})
	// MFA: o AdvPP não faz segundo fator (sessão local; o REST não o
	// exigiria). hasSecret=false faz a lib-core pular o fluxo
	// (ProMfaHasSecretInterface); nenhum segredo é gerado nem gravado.
	get("/api/framework/mfa/hasSecret", func(r *rest.Request) *rest.Response {
		return fwJSON(map[string]any{"hasSecret": false})
	})
	get("/api/framework/mfa/tok", func(r *rest.Request) *rest.Response {
		return fwJSON(map[string]any{"tok": 0})
	})
	get("/api/framework/mfa/TOTPURI", func(r *rest.Request) *rest.Response {
		return fwErro(501, "MFA não suportado no AdvPP (sessão local)")
	})
}

// fwToken emite o token de sessão local. O access_token é um JWT
// "header.payload." (sem assinatura — o cliente só faz atob do payload)
// com userid/exp, como o Protheus emite; refresh_token é opaco.
func fwToken() *rest.Response {
	_, _, _, usuario := frameworkSessao()
	now := time.Now().Unix()
	exp := now + 3600
	b64 := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	jwt := b64(map[string]string{"alg": "none", "typ": "JWT"}) + "." +
		b64(map[string]any{"userid": usuario, "iat": now, "exp": exp}) + "."
	return fwJSON(map[string]any{
		"access_token":  jwt,
		"refresh_token": jwt,
		"expires_in":    3600,
		"token_type":    "Bearer",
	})
}
