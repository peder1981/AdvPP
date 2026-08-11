package vm

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// --- XmlC14N ---

func TestXmlC14NExemploTDN(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cXml := `<?xml version="1.0" encoding="UTF-8"?>
<!-- comentario deve sumir -->
<pedido attrB="2" attrA="1">
  <item><![CDATA[texto & cdata]]></item>
  <vazio/>
</pedido>`
	got, err := v.natives["XMLC14N"].Fn([]advplrt.Value{
		advplrt.NewString(cXml),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlC14N retornou erro: %v", err)
	}
	s, ok := got.(*advplrt.StringValue)
	if !ok {
		t.Fatalf("XmlC14N retornou tipo %T, quer StringValue", got)
	}
	out := s.Val
	if out == "" {
		t.Fatalf("XmlC14N retornou string vazia para XML válido")
	}
	// comentário deve ter sido removido
	if contains(out, "comentario") {
		t.Errorf("comentário não removido: %s", out)
	}
	// CDATA vira conteúdo explícito (sem marcador CDATA)
	if contains(out, "CDATA") {
		t.Errorf("marcador CDATA não removido: %s", out)
	}
	if !contains(out, "texto &amp; cdata") {
		t.Errorf("conteúdo do CDATA não preservado/escapado corretamente: %s", out)
	}
	// atributos devem sair ordenados (attrA antes de attrB)
	idxA := indexOf(out, "attrA")
	idxB := indexOf(out, "attrB")
	if idxA < 0 || idxB < 0 || idxA > idxB {
		t.Errorf("atributos não ordenados corretamente: %s", out)
	}
	// elemento vazio nunca em forma self-closing
	if contains(out, "<vazio/>") {
		t.Errorf("elemento vazio não deveria usar self-closing: %s", out)
	}
	if !contains(out, "<vazio></vazio>") {
		t.Errorf("elemento vazio deveria virar <vazio></vazio>: %s", out)
	}
}

func TestXmlC14NDocumentoVazio(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLC14N"].Fn([]advplrt.Value{
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlC14N retornou erro: %v", err)
	}
	s, ok := got.(*advplrt.StringValue)
	if !ok || s.Val != "" {
		t.Errorf("XmlC14N(\"\") = %v, quer string vazia (TDN: 'Invalid empty document on XmlC14N')", got)
	}
}

func TestXmlC14NMalFormado(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLC14N"].Fn([]advplrt.Value{
		advplrt.NewString("<a><b></a>"),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlC14N retornou erro: %v", err)
	}
	s, ok := got.(*advplrt.StringValue)
	if !ok || s.Val != "" {
		t.Errorf("XmlC14N(malformado) = %v, quer string vazia (TDN: 'Failed to parse XML')", got)
	}
}

// --- XmlC14NFile ---

func TestXmlC14NFileExemploTDN(t *testing.T) {
	// XmlC14NFile converte cFile para minúsculas antes de ler (comportamento
	// documentado na TDN) — usamos um diretório já em minúsculas para não
	// colidir com essa conversão em filesystems case-sensitive (Linux).
	dir := filepath.Join(os.TempDir(), "advpp_xmlc14nfile_test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("falha ao criar dir de fixture: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "example.xml")
	if err := os.WriteFile(path, []byte(`<pedido><item>1</item></pedido>`), 0644); err != nil {
		t.Fatalf("falha ao escrever fixture: %v", err)
	}
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLC14NFILE"].Fn([]advplrt.Value{
		advplrt.NewString(path),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlC14NFile retornou erro: %v", err)
	}
	s, ok := got.(*advplrt.StringValue)
	if !ok || s.Val == "" {
		t.Fatalf("XmlC14NFile(%q) retornou %v, quer XML canonicalizado não vazio", path, got)
	}
	if !contains(s.Val, "<pedido>") {
		t.Errorf("XmlC14NFile não canonicalizou corretamente: %s", s.Val)
	}
}

func TestXmlC14NFileArquivoInexistente(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLC14NFILE"].Fn([]advplrt.Value{
		advplrt.NewString("/caminho/que/nao/existe.xml"),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlC14NFile retornou erro: %v", err)
	}
	s, ok := got.(*advplrt.StringValue)
	if !ok || s.Val != "" {
		t.Errorf("XmlC14NFile(arquivo inexistente) = %v, quer string vazia", got)
	}
}

func TestXmlC14NFileSmartClientPathRejeitado(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Windows 'C:\\...' é caminho de servidor válido (não é rejeitado)")
	}
	v := NewVM(&compiler.Bytecode{}, false)
	_, err := v.natives["XMLC14NFILE"].Fn([]advplrt.Value{
		advplrt.NewString(`C:\temp\example.xml`),
		advplrt.NewString(""),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err == nil {
		t.Fatalf("XmlC14NFile com caminho de SmartClient deveria interromper com erro (TDN: 'Only server path are allowed on XmlC14NFile')")
	}
}

// --- XmlFVldSch ---

func TestXmlFVldSchXmlValidoBemFormado(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "valid.xml")
	xsdPath := filepath.Join(dir, "schema_definition.xsd")
	os.WriteFile(xmlPath, []byte(`<pedido><Quantidade>1</Quantidade></pedido>`), 0644)
	os.WriteFile(xsdPath, []byte(`<?xml version="1.0"?><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`), 0644)

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLFVLDSCH"].Fn([]advplrt.Value{
		advplrt.NewString(xmlPath),
		advplrt.NewString(xsdPath),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlFVldSch retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || !b.Val {
		t.Errorf("XmlFVldSch(xml e xsd bem-formados, schema sem elementos de topo) = %v, quer .T. (schema sem forma reconhecida cai em bem-formação apenas, ver limitação documentada)", got)
	}
}

// TestXmlFVldSchExemploTDNQuantidadeXsInteger reproduz o cenário descrito
// literalmente na própria página XmlFVldSch.md: um schema que tipa
// <Quantidade> como xs:integer, validado contra um XML com Quantidade
// numérico (deve passar, .T.) e contra um XML com Quantidade='ABC' (deve
// reprovar, .F. — a TDN documenta a mensagem de erro exata: "Element
// 'Quantidade': 'ABC' is not a valid value of the atomic type
// 'xs:integer'."). Este é o teste de regressão para o bug encontrado em
// code review: uma versão anterior desta função só checava boa-formação e
// retornava .T. para os dois casos, inclusive o inválido.
func TestXmlFVldSchExemploTDNQuantidadeXsInteger(t *testing.T) {
	dir := t.TempDir()
	xsdPath := filepath.Join(dir, "schema_definition.xsd")
	os.WriteFile(xsdPath, []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="pedido">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="Quantidade" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`), 0644)

	v := NewVM(&compiler.Bytecode{}, false)

	validPath := filepath.Join(dir, "valid.xml")
	os.WriteFile(validPath, []byte(`<pedido><Quantidade>1</Quantidade></pedido>`), 0644)
	got, err := v.natives["XMLFVLDSCH"].Fn([]advplrt.Value{
		advplrt.NewString(validPath),
		advplrt.NewString(xsdPath),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlFVldSch(valid.xml) retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || !b.Val {
		t.Errorf("XmlFVldSch(valid.xml, Quantidade=1 contra xs:integer) = %v, quer .T.", got)
	}

	invalidPath := filepath.Join(dir, "invalid.xml")
	os.WriteFile(invalidPath, []byte(`<pedido><Quantidade>ABC</Quantidade></pedido>`), 0644)
	got, err = v.natives["XMLFVLDSCH"].Fn([]advplrt.Value{
		advplrt.NewString(invalidPath),
		advplrt.NewString(xsdPath),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlFVldSch(invalid.xml) retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || b.Val {
		t.Errorf("XmlFVldSch(invalid.xml, Quantidade='ABC' contra xs:integer) = %v, quer .F. (TDN: \"Element 'Quantidade': 'ABC' is not a valid value of the atomic type 'xs:integer'.\")", got)
	}
}

// TestXmlFVldSchElementoObrigatorioAusente cobre o outro mecanismo do
// verificador mínimo: presença de elemento obrigatório (minOccurs padrão 1).
func TestXmlFVldSchElementoObrigatorioAusente(t *testing.T) {
	dir := t.TempDir()
	xsdPath := filepath.Join(dir, "schema.xsd")
	os.WriteFile(xsdPath, []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="pedido">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="Quantidade" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`), 0644)
	xmlPath := filepath.Join(dir, "sem_quantidade.xml")
	os.WriteFile(xmlPath, []byte(`<pedido></pedido>`), 0644)

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLFVLDSCH"].Fn([]advplrt.Value{
		advplrt.NewString(xmlPath),
		advplrt.NewString(xsdPath),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlFVldSch retornou erro: %v", err)
	}
	if b, ok := got.(*advplrt.BoolValue); !ok || b.Val {
		t.Errorf("XmlFVldSch(pedido sem Quantidade obrigatória) = %v, quer .F.", got)
	}
}

func TestXmlFVldSchXmlMalFormado(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "invalid.xml")
	xsdPath := filepath.Join(dir, "schema_definition.xsd")
	os.WriteFile(xmlPath, []byte(`<pedido><Quantidade>1</pedido>`), 0644) // mal-formado (tag não fecha)
	os.WriteFile(xsdPath, []byte(`<?xml version="1.0"?><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`), 0644)

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLFVLDSCH"].Fn([]advplrt.Value{
		advplrt.NewString(xmlPath),
		advplrt.NewString(xsdPath),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlFVldSch retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || b.Val {
		t.Errorf("XmlFVldSch(xml mal-formado) = %v, quer .F.", got)
	}
}

func TestXmlFVldSchArquivoInexistente(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLFVLDSCH"].Fn([]advplrt.Value{
		advplrt.NewString("/nao/existe.xml"),
		advplrt.NewString("/nao/existe.xsd"),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlFVldSch retornou erro: %v", err)
	}
	b, ok := got.(*advplrt.BoolValue)
	if !ok || b.Val {
		t.Errorf("XmlFVldSch(arquivos inexistentes) = %v, quer .F.", got)
	}
}

// --- XmlParser ---

func TestXmlParserExemploTDN(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	cXml := `<?xml version="1.0" encoding="ISO-8859-1"?>` +
		`<pedido>` +
		`    <NomeCliente>Microsiga Software</NomeCliente>` +
		`    <Endereco>Av. Braz Leme</Endereco>` +
		`    <Itens>` +
		`        <Item><Produto>Protheus</Produto></Item>` +
		`        <Item><Produto>Outro</Produto></Item>` +
		`    </Itens>` +
		`</pedido>`
	got, err := v.natives["XMLPARSER"].Fn([]advplrt.Value{
		advplrt.NewString(cXml),
		advplrt.NewString("_"),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlParser retornou erro: %v", err)
	}
	doc, ok := got.(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("XmlParser retornou %T, quer ObjectValue (oXml)", got)
	}
	pedido, ok := doc.Props["_PEDIDO"].(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("oXml:_PEDIDO ausente ou não é objeto; props=%v", doc.Keys)
	}
	nome, ok := pedido.Props["_NOMECLIENTE"].(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("oXml:_PEDIDO:_NOMECLIENTE ausente; props=%v", pedido.Keys)
	}
	text, ok := nome.Props["TEXT"].(*advplrt.StringValue)
	if !ok || text.Val != "Microsiga Software" {
		t.Errorf("oXml:_PEDIDO:_NOMECLIENTE:Text = %v, quer 'Microsiga Software'", nome.Props["TEXT"])
	}
	realname, ok := nome.Props["REALNAME"].(*advplrt.StringValue)
	if !ok || realname.Val != "NomeCliente" {
		t.Errorf("REALNAME = %v, quer 'NomeCliente'", nome.Props["REALNAME"])
	}
	// Itens repetidos (<Item> aparece 2x) devem cair em ARRAYNODES, não em _ITEM escalar.
	itens, ok := pedido.Props["_ITENS"].(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("oXml:_PEDIDO:_ITENS ausente")
	}
	arr, ok := itens.Props["ARRAYNODES"].(*advplrt.ArrayValue)
	if !ok || len(arr.Elements) != 2 {
		t.Errorf("ARRAYNODES = %v, quer array com 2 elementos (Item repetido)", itens.Props["ARRAYNODES"])
	}
}

func TestXmlParserXmlVazioRetornaNil(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLPARSER"].Fn([]advplrt.Value{
		advplrt.NewString(""),
		advplrt.NewString("_"),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlParser retornou erro: %v", err)
	}
	if _, isNil := got.(*advplrt.NilValue); !isNil {
		t.Errorf("XmlParser(\"\") = %T, quer NilValue", got)
	}
}

func TestXmlParserMalFormadoRetornaNil(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLPARSER"].Fn([]advplrt.Value{
		advplrt.NewString("<a><b></a>"),
		advplrt.NewString("_"),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlParser retornou erro: %v", err)
	}
	if _, isNil := got.(*advplrt.NilValue); !isNil {
		t.Errorf("XmlParser(malformado) = %T, quer NilValue", got)
	}
}

// --- XmlParserFile ---

func TestXmlParserFileExemplo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pedido.xml")
	os.WriteFile(path, []byte(`<pedido><Cliente>ACME</Cliente></pedido>`), 0644)

	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLPARSERFILE"].Fn([]advplrt.Value{
		advplrt.NewString(path),
		advplrt.NewString("_"),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlParserFile retornou erro: %v", err)
	}
	doc, ok := got.(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("XmlParserFile retornou %T, quer ObjectValue", got)
	}
	pedido, ok := doc.Props["_PEDIDO"].(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("oXml:_PEDIDO ausente")
	}
	cliente, ok := pedido.Props["_CLIENTE"].(*advplrt.ObjectValue)
	if !ok {
		t.Fatalf("oXml:_PEDIDO:_CLIENTE ausente")
	}
	if text, ok := cliente.Props["TEXT"].(*advplrt.StringValue); !ok || text.Val != "ACME" {
		t.Errorf("TEXT = %v, quer 'ACME'", cliente.Props["TEXT"])
	}
}

func TestXmlParserFileArquivoInexistenteRetornaNil(t *testing.T) {
	v := NewVM(&compiler.Bytecode{}, false)
	got, err := v.natives["XMLPARSERFILE"].Fn([]advplrt.Value{
		advplrt.NewString("/nao/existe.xml"),
		advplrt.NewString("_"),
		advplrt.NewString(""),
		advplrt.NewString(""),
	})
	if err != nil {
		t.Fatalf("XmlParserFile retornou erro: %v", err)
	}
	if _, isNil := got.(*advplrt.NilValue); !isNil {
		t.Errorf("XmlParserFile(arquivo inexistente) = %T, quer NilValue", got)
	}
}

// --- helpers ---

func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
