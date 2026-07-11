package vm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// BrowseUI é a extensão opcional de UIProvider para renderizar um browse
// (FWMBrowse) — implementada pelo modo web (advplc serve). Recebe o spec em
// JSON, bloqueia até o usuário agir e devolve a ação (também JSON).
type BrowseUI interface {
	Browse(spec []byte) []byte
}

// SQLEngine é a extensão opcional de DBEngine com acesso SQL direto —
// usada pelo browse para ler o dicionário SX3 e executar o CRUD.
type SQLEngine interface {
	QueryRows(query string, args ...any) ([]map[string]string, error)
	Exec(query string, args ...any) error
}

// browseState é o estado Go da classe FWMBrowse (campo Native do objeto).
type browseState struct {
	alias string
	title string
}

type browseColumn struct {
	Property string `json:"property"`
	Label    string `json:"label"`
	Type     string `json:"type"` // C | N | D | L | M (tipos SX3)
	Size     int    `json:"size"`
	Decimal  int    `json:"decimal"`
}

type browseSpec struct {
	Title   string           `json:"title"`
	Alias   string           `json:"alias"`
	Columns []browseColumn   `json:"columns"`
	Items   []map[string]any `json:"items"`
}

type browseAction struct {
	Action string            `json:"action"` // save | delete | close
	Recno  int64             `json:"recno"`  // rowid; 0 = inclusão
	Data   map[string]string `json:"data"`
}

var identRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func newBrowseObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("FWMBrowse", nil)
	obj.Native = &browseState{}
	return obj
}

func (v *VM) callFormBrowseMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	b, ok := obj.Native.(*browseState)
	if !ok {
		return fmt.Errorf("FWMBrowse: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		v.push(obj)

	case "SETALIAS":
		b.alias = strings.ToUpper(strings.TrimSpace(advplrt.ToString(getArg(args, 0))))
		v.push(advplrt.Nil)

	case "SETDESCRIPTION", "SETTITLE":
		b.title = advplrt.ToString(getArg(args, 0))
		v.push(advplrt.Nil)

	case "ACTIVATE":
		if err := v.runBrowse(b); err != nil {
			return err
		}
		v.push(advplrt.Nil)

	case "DEACTIVATE", "DESTROY":
		v.push(advplrt.Nil)

	default:
		return fmt.Errorf("unknown method %s on FWMBrowse", method)
	}
	return nil
}

// runBrowse executa o ciclo do browse: monta colunas (SX3) + linhas, envia
// à UI e aplica a ação devolvida (CRUD) até o usuário fechar.
func (v *VM) runBrowse(b *browseState) error {
	ui, ok := v.uiProvider.(BrowseUI)
	if !ok {
		return fmt.Errorf("FWMBrowse: requer o modo web (advplc serve)")
	}
	sqlEng, ok := v.dbEngine.(SQLEngine)
	if !ok || v.dbEngine == nil {
		return fmt.Errorf("FWMBrowse: nenhum banco de dados conectado")
	}
	if !identRe.MatchString(b.alias) {
		return fmt.Errorf("FWMBrowse: alias inválido %q", b.alias)
	}

	cols, hasDelete, err := v.browseColumns(sqlEng, b.alias)
	if err != nil {
		return err
	}

	for {
		items, err := browseItems(sqlEng, b.alias, cols, hasDelete)
		if err != nil {
			return err
		}
		title := b.title
		if title == "" {
			title = b.alias
		}
		spec, _ := json.Marshal(browseSpec{Title: title, Alias: b.alias, Columns: cols, Items: items})

		var act browseAction
		if err := json.Unmarshal(ui.Browse(spec), &act); err != nil {
			return nil // resposta inválida/sessão encerrada: fecha o browse
		}

		switch act.Action {
		case "save":
			if err := browseSave(sqlEng, b.alias, cols, hasDelete, act); err != nil {
				return err
			}
		case "delete":
			if err := browseDelete(sqlEng, b.alias, hasDelete, act.Recno); err != nil {
				return err
			}
		default: // close
			return nil
		}
	}
}

// browseColumns monta as colunas a partir do dicionário SX3, limitadas às
// colunas físicas da tabela. Sem SX3, usa as colunas físicas (fallback).
func (v *VM) browseColumns(eng SQLEngine, alias string) ([]browseColumn, bool, error) {
	phys, err := eng.QueryRows(fmt.Sprintf("PRAGMA table_info(%s)", alias))
	if err != nil || len(phys) == 0 {
		return nil, false, fmt.Errorf("FWMBrowse: tabela %s não encontrada", alias)
	}
	physSet := map[string]bool{}
	hasDelete := false
	for _, p := range phys {
		name := strings.ToUpper(p["NAME"])
		physSet[name] = true
		if name == "D_E_L_E_T_" {
			hasDelete = true
		}
	}

	cols := []browseColumn{}
	sx3, err := eng.QueryRows(
		"SELECT X3_CAMPO, X3_TITULO, X3_TIPO, X3_TAMANHO, X3_DECIMAL FROM SX3 WHERE UPPER(X3_ARQUIVO) = ? ORDER BY X3_ORDEM",
		alias)
	if err == nil {
		for _, f := range sx3 {
			campo := strings.ToUpper(strings.TrimSpace(f["X3_CAMPO"]))
			if !physSet[campo] || !identRe.MatchString(campo) {
				continue
			}
			size, _ := strconv.Atoi(f["X3_TAMANHO"])
			dec, _ := strconv.Atoi(f["X3_DECIMAL"])
			label := strings.TrimSpace(f["X3_TITULO"])
			if label == "" {
				label = campo
			}
			cols = append(cols, browseColumn{
				Property: campo, Label: label,
				Type: strings.TrimSpace(f["X3_TIPO"]), Size: size, Decimal: dec,
			})
		}
	}
	if len(cols) == 0 { // sem entradas SX3: colunas físicas como caracter
		for _, p := range phys {
			name := strings.ToUpper(p["NAME"])
			if name == "D_E_L_E_T_" || !identRe.MatchString(name) {
				continue
			}
			cols = append(cols, browseColumn{Property: name, Label: name, Type: "C"})
		}
	}
	return cols, hasDelete, nil
}

func browseItems(eng SQLEngine, alias string, cols []browseColumn, hasDelete bool) ([]map[string]any, error) {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Property
	}
	// "rowid" is aliased explicitly (not selected bare) because SQLite
	// reports the RESULT column's name as the table's own INTEGER PRIMARY
	// KEY alias when it has one — every AdvPP-managed table does
	// (R_E_C_N_O_, see the logical-delete convention) — not literally
	// "rowid", which silently broke the lookup below (recno always came
	// back as 0, turning every edit into a duplicate INSERT instead of an
	// UPDATE).
	query := fmt.Sprintf("SELECT rowid AS browse_recno_, %s FROM %s", strings.Join(names, ", "), alias)
	if hasDelete {
		query += " WHERE D_E_L_E_T_ <> '*'"
	}
	rows, err := eng.QueryRows(query)
	if err != nil {
		return nil, fmt.Errorf("FWMBrowse: %w", err)
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		item := map[string]any{}
		recno, _ := strconv.ParseInt(r["BROWSE_RECNO_"], 10, 64)
		item["recno"] = recno
		for _, c := range cols {
			val := r[c.Property]
			if c.Type == "N" {
				n, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
				item[c.Property] = n
			} else {
				item[c.Property] = val
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func browseSave(eng SQLEngine, alias string, cols []browseColumn, hasDelete bool, act browseAction) error {
	names := []string{}
	vals := []any{}
	for _, c := range cols {
		raw, ok := act.Data[c.Property]
		if !ok {
			continue
		}
		names = append(names, c.Property)
		if c.Type == "N" {
			n, _ := strconv.ParseFloat(strings.TrimSpace(raw), 64)
			vals = append(vals, n)
		} else {
			vals = append(vals, raw)
		}
	}
	if len(names) == 0 {
		return nil
	}
	if act.Recno == 0 {
		if hasDelete {
			names = append(names, "D_E_L_E_T_")
			vals = append(vals, " ")
		}
		q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", alias,
			strings.Join(names, ", "), strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", "))
		return eng.Exec(q, vals...)
	}
	sets := make([]string, len(names))
	for i, n := range names {
		sets[i] = n + " = ?"
	}
	vals = append(vals, act.Recno)
	return eng.Exec(fmt.Sprintf("UPDATE %s SET %s WHERE rowid = ?", alias, strings.Join(sets, ", ")), vals...)
}

func browseDelete(eng SQLEngine, alias string, hasDelete bool, recno int64) error {
	if recno == 0 {
		return nil
	}
	if hasDelete { // soft-delete padrão Protheus
		return eng.Exec(fmt.Sprintf("UPDATE %s SET D_E_L_E_T_ = '*' WHERE rowid = ?", alias), recno)
	}
	return eng.Exec(fmt.Sprintf("DELETE FROM %s WHERE rowid = ?", alias), recno)
}
