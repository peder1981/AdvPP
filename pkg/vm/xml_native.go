package vm

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
)

// registerTratamentodeXMLNatives registra as funções TDN da categoria
// "Tratamento de XML" que possuem especificação real (5 de 13 no índice do
// mirror): XmlC14N, XmlC14NFile, XmlFVldSch, XmlParser, XmlParserFile. As
// outras 8 (XmlDelNode, XmlNode2Arr, XmlSVldSch, XmlChildEx, XmlCloneNode,
// XmlNewNode, XmlGetChild, XmlGetParent) estão no ledger de gaps
// (docs/tdn-gap-stubs.md) — sem página com Sintaxe/Parâmetros real, não
// implementadas aqui.
//
// LIMITAÇÃO ARQUITETURAL (ver docs/tdn-known-limitations.md, seção
// "Parâmetros por referência"): todas as 5 funções desta categoria têm, na
// TDN, os últimos 1-2 parâmetros como saída por referência (@cError,
// @cWarning). O VM do AdvPP não muta variáveis do chamador passadas com @
// (mesma limitação de IPCWaitEx/SFTP*/GetFuncArray). Os parâmetros cError/
// cWarning são aceitos posicionalmente mas nunca escritos de volta; o valor
// de retorno principal de cada função é a forma suportada de checar
// sucesso/falha, e é isso que os testes desta categoria verificam.
//
// XmlC14N/XmlC14NFile: a canonicalização W3C REC-xml-c14n-20010315 é
// implementada de fato (não é stub) cobrindo o núcleo do algoritmo:
// remoção de comentários e da declaração/PIs, CDATA convertido para
// conteúdo literal, ordenação de atributos e declarações de namespace,
// elementos vazios sempre com tag de fechamento explícita (nunca
// self-closing) e escaping de caracteres conforme a especificação (texto:
// &,<,>,CR; atributo: &,<,",TAB,LF,CR). NÃO implementado (documentado, não
// disfarçado): o eixo de namespace completo do C14N não-exclusivo (um
// namespace herdado de um ancestral distante, sem xmlns local em nenhum
// elemento no caminho, não é re-renderizado no descendente — este
// compilador só resolve prefixos que aparecem como atributo xmlns em algum
// elemento realmente visitado na pilha), atributos xml:* especiais (xml:lang,
// xml:space) não recebem tratamento de herança dedicado, e processing
// instructions que não sejam a declaração XML são descartadas (o C14N real
// as preserva). Nenhum destes casos aparece nos exemplos da própria página
// TDN (que não inclui o conteúdo do arquivo de exemplo, apenas o chama).
//
// XmlFVldSch: AdvPP não tem, nem no stdlib nem em go.mod, um validador de
// XML Schema (XSD) completo — go.mod não lista nenhuma dependência de
// schema/xmlsec/c14n (conferido). Um validador XSD 1.1 completo (xs:choice,
// xs:pattern, tipos simples derivados por restrição, etc.) é um projeto por
// si só, fora do escopo desta task. Em vez de tratar a função inteira como
// fora de escopo, foi implementado um verificador estrutural mínimo mas
// real (xsdCheckSchema/xsdValidateInstance, mais abaixo neste arquivo) que
// cobre exatamente o cenário do próprio exemplo da TDN: presença de
// elementos obrigatórios segundo xs:sequence/minOccurs, e verificação de
// tipo primitivo (xs:integer, xs:decimal, xs:boolean, xs:date, xs:dateTime,
// xs:string) do conteúdo de cada elemento tipado. Isso já resolve
// corretamente o caso descrito na própria página do TDN — um schema que
// tipa <Quantidade> como xs:integer deve reprovar um XML com
// <Quantidade>ABC</Quantidade> — que uma versão anterior desta função
// (apenas checagem de boa-formação) respondia errado (retornava .T.). O que
// continua fora de escopo, documentado (não fingido): xs:choice/xs:all,
// xs:pattern/xs:enumeration, tipos simples nomeados derivados por
// restrição, complexType referenciado com prefixo de namespace não-trivial,
// e atributos XML (só elementos são checados). Quando o schema usa algum
// desses recursos não suportados, ou quando a forma raiz do XML não bate
// com nenhum <xs:element> de topo do schema, a checagem estrutural não
// consegue se aplicar e a função cai de volta em "apenas bem-formado" (ver
// xsdCheckSchema) — documentado em detalhe em
// docs/tdn-known-limitations.md.
//
// XmlParser/XmlParserFile: função descontinuada segundo a própria TDN
// ("recomenda-se tXmlManager"), mas com especificação real de estrutura de
// retorno (objeto dinâmico por node XML, com REALNAME/TEXT/TYPE e
// propriedades filhas nomeadas <cReplace><NodeName> em maiúsculas).
// Implementado usando encoding/xml (parser real, não simulado) construindo
// a árvore de advplrt.ObjectValue dinamicamente — sem reimplementar a
// infraestrutura completa de manipulação de node (XmlNewNode/XmlGetChild/
// XmlGetParent/etc. são gaps documentados, não construídos aqui pois não
// têm especificação própria). Quando um node tem mais de um filho com o
// mesmo nome de tag, os filhos repetidos são agrupados na propriedade
// ARRAYNODES (um advplrt.ArrayValue) em vez de props escalares individuais
// — interpretação 🟡 INFERIDA da estrutura "<ArrayNodes>" mostrada no
// diagrama da TDN (a página não traz um exemplo executável demonstrando
// esse caso). Nota: o exemplo da própria TDN mostra a navegação
// "oXml:_PEDIDO:_NOMECLIENTE" para o node <Nome_Cliente> — sem o underscore
// interno, o que é inconsistente com underscore ser caractere válido de
// identificador AdvPL (não deveria ser substituído por cReplace). Tratado
// como 🔴 AMBÍGUO/possível erro de digitação da própria doc: esta
// implementação preserva o underscore original (gera "_NOME_CLIENTE"),
// comportamento mais consistente com a regra descrita de cReplace
// substituir apenas caracteres inválidos.
//
// XmlParserFile especificamente: a página do mirror para esta função não
// tem seção de Sintaxe/Parâmetros/Retorno/Exemplo (apenas o aviso de
// descontinuação) — 🟡 INFERIDO por analogia com o par
// XmlC14N/XmlC14NFile (mesma assinatura de XmlParser, trocando cXml por
// cFile, lendo do disco).
func (v *VM) registerTratamentodeXMLNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// XmlC14N( cXML, cOption, @cError, @cWarning ) -> cRetXML
	// Aplica o algoritmo de canonicalização C14N (W3C REC-xml-c14n-20010315)
	// na string XML informada. Retorna "" se cXML for vazio ou mal-formado
	// (TDN: mensagens "Invalid empty document on XmlC14N" / "Failed to parse
	// XML" seriam escritas em @cError, mas ver limitação de by-ref acima).
	natives["XMLC14N"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cXML := getArgString(args, 0, "")
		if strings.Trim(cXML, " ") == "" {
			return advplrt.NewString(""), nil
		}
		out, err := canonicalizeXML(cXML)
		if err != nil {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(out), nil
	}

	// XmlC14NFile( cFile, cOption, @cError, @cWarning ) -> cRetXML
	// Mesma canonicalização de XmlC14N, lendo o XML de um arquivo em disco
	// (caminho relativo ao cwd do processo AdvPP, análogo a GetAPOInfo em
	// rpo_native.go). O caminho é convertido para minúsculas antes da leitura
	// (comportamento documentado na TDN, herdado da semântica case-insensitive
	// de Windows — em filesystems case-sensitive como Linux isto pode impedir
	// de achar arquivos com maiúsculas no nome; comportamento replicado como
	// documentado, não é bug introduzido por este compilador). Caminhos no
	// formato de SmartClient (ex.: "C:\...") interrompem a execução com erro,
	// conforme TDN ("Only server path are allowed on XmlC14NFile"). Essa
	// rejeição só se aplica fora do Windows: no Windows, "C:\..." é um caminho
	// de servidor válido (é o próprio SO onde o appserver roda).
	natives["XMLC14NFILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFile := strings.Trim(getArgString(args, 0, ""), " ")
		if cFile == "" {
			return advplrt.NewString(""), nil
		}
		if runtime.GOOS != "windows" && smartClientPathPattern.MatchString(cFile) {
			return advplrt.Nil, fmt.Errorf("Only server path are allowed on XmlC14NFile")
		}
		data, err := os.ReadFile(strings.ToLower(cFile))
		if err != nil {
			return advplrt.NewString(""), nil
		}
		out, err := canonicalizeXML(string(data))
		if err != nil {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(out), nil
	}

	// XmlFVldSch( cXML, cXSD, @cError, @cWarning ) -> lRetorno
	// Valida um arquivo XML contra um XSD. Checagens reais: os dois arquivos
	// existem e são legíveis, ambos são XML bem-formado, e — quando a forma
	// do schema é reconhecida (ver xsdCheckSchema) — presença de elementos
	// obrigatórios + tipo primitivo do conteúdo de cada elemento tipado.
	// Cobre o exemplo real da própria TDN (Quantidade tipado xs:integer
	// reprovando conteúdo 'ABC'). Ver nota de limitação acima para o que
	// permanece fora de escopo (xs:choice, xs:pattern, etc.).
	natives["XMLFVLDSCH"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cXML := strings.Trim(getArgString(args, 0, ""), " ")
		cXSD := strings.Trim(getArgString(args, 1, ""), " ")
		if cXML == "" || cXSD == "" {
			return advplrt.False, nil
		}
		xmlData, err := os.ReadFile(cXML)
		if err != nil {
			return advplrt.False, nil
		}
		xsdData, err := os.ReadFile(cXSD)
		if err != nil {
			return advplrt.False, nil
		}
		if !isWellFormedXML(xmlData) || !isWellFormedXML(xsdData) {
			return advplrt.False, nil
		}
		return advplrt.NewBool(xsdCheckSchema(xmlData, xsdData)), nil
	}

	// XmlParser( [cXml], [cReplace], @cError, @cWarning ) -> oXml
	// Retorna um objeto dinâmico representando a estrutura do XML informado.
	// Retorna NIL se cXml for vazio ou mal-formado (função descontinuada
	// segundo a própria TDN; ver nota de limitação/inferência acima).
	natives["XMLPARSER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cXml := getArgString(args, 0, "")
		cReplace := getArgString(args, 1, "_")
		if strings.Trim(cXml, " ") == "" {
			return advplrt.Nil, nil
		}
		doc, err := xmlParseDocument(cXml, cReplace)
		if err != nil {
			return advplrt.Nil, nil
		}
		return doc, nil
	}

	// XmlParserFile( [cFile], [cReplace], @cError, @cWarning ) -> oXml
	// Mesma estrutura de retorno de XmlParser, lendo o XML de um arquivo em
	// disco (caminho relativo ao cwd, análogo a XmlC14NFile/GetAPOInfo).
	// Assinatura 🟡 INFERIDA por analogia (a página TDN não documenta
	// Sintaxe/Parâmetros — ver nota acima).
	natives["XMLPARSERFILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFile := strings.Trim(getArgString(args, 0, ""), " ")
		cReplace := getArgString(args, 1, "_")
		if cFile == "" {
			return advplrt.Nil, nil
		}
		data, err := os.ReadFile(cFile)
		if err != nil {
			return advplrt.Nil, nil
		}
		doc, err := xmlParseDocument(string(data), cReplace)
		if err != nil {
			return advplrt.Nil, nil
		}
		return doc, nil
	}
}

// newXMLDecoder cria um xml.Decoder com CharsetReader configurado: fontes
// AdvPL/Protheus frequentemente declaram encoding não-UTF-8 (mais comum:
// ISO-8859-1/windows-1252, coerente com a convenção CP-1252 deste próprio
// compilador para .prw/.tlpp). O stdlib encoding/xml rejeita qualquer
// documento com encoding declarado != UTF-8 sem CharsetReader configurado
// ("declared but Decoder.CharsetReader is nil"); resolvemos o nome do
// charset via golang.org/x/text/encoding/htmlindex (aceita os aliases
// comuns: iso-8859-1, windows-1252, latin1 etc.) e decodificamos de fato
// (sem chute) via golang.org/x/text — ambos já dependências existentes do
// módulo (golang.org/x/text é dependência indireta pré-existente).
func newXMLDecoder(r io.Reader) *xml.Decoder {
	dec := xml.NewDecoder(r)
	dec.Strict = true
	dec.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		enc, err := htmlindex.Get(charset)
		if err != nil {
			// fallback: ISO-8859-1 é o encoding declarado mais comum em fontes
			// Protheus legadas quando o charset não é reconhecido pelo índice.
			enc = charmap.ISO8859_1
		}
		return enc.NewDecoder().Reader(input), nil
	}
	return dec
}

// smartClientPathPattern detecta um caminho absoluto de Windows (padrão de
// caminho do SmartClient, ex.: "C:\temp\arq.xml"), que XmlC14NFile rejeita
// segundo a TDN ("Only server path are allowed on XmlC14NFile").
var smartClientPathPattern = regexp.MustCompile(`^[A-Za-z]:\\`)

// isWellFormedXML checa se data é um documento XML sintaticamente válido
// (percorre todos os tokens até EOF sem erro). Não avalia semântica de
// schema — apenas boa-formação sintática.
func isWellFormedXML(data []byte) bool {
	dec := newXMLDecoder(strings.NewReader(string(data)))
	sawElement := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return sawElement
		}
		if err != nil {
			return false
		}
		if _, ok := tok.(xml.StartElement); ok {
			sawElement = true
		}
	}
}

// --- XmlC14N: canonicalização W3C C14N 1.0 (não-exclusiva, sem comentários) ---

type xmlNsDecl struct{ prefix, uri string }

// canonicalizeXML aplica o núcleo do algoritmo C14N 1.0 na string XML dada.
// Ver nota de limitação no topo do arquivo para o que não é coberto.
func canonicalizeXML(src string) (string, error) {
	dec := newXMLDecoder(strings.NewReader(src))

	var out strings.Builder
	var nsStack [][]xmlNsDecl
	sawRoot := false

	resolvePrefix := func(uri string) string {
		for i := len(nsStack) - 1; i >= 0; i-- {
			for _, d := range nsStack[i] {
				if d.uri == uri {
					return d.prefix
				}
			}
		}
		return ""
	}
	qname := func(n xml.Name) string {
		if n.Space == "" {
			return n.Local
		}
		if p := resolvePrefix(n.Space); p != "" {
			return p + ":" + n.Local
		}
		return n.Local
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			sawRoot = true
			var decls []xmlNsDecl
			var attrs []xml.Attr
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" {
					decls = append(decls, xmlNsDecl{a.Name.Local, a.Value})
				} else if a.Name.Space == "" && a.Name.Local == "xmlns" {
					decls = append(decls, xmlNsDecl{"", a.Value})
				} else {
					attrs = append(attrs, a)
				}
			}
			nsStack = append(nsStack, decls)
			sort.Slice(decls, func(i, j int) bool { return decls[i].prefix < decls[j].prefix })
			sort.Slice(attrs, func(i, j int) bool {
				if attrs[i].Name.Space != attrs[j].Name.Space {
					return attrs[i].Name.Space < attrs[j].Name.Space
				}
				return attrs[i].Name.Local < attrs[j].Name.Local
			})

			out.WriteByte('<')
			out.WriteString(qname(t.Name))
			for _, d := range decls {
				out.WriteByte(' ')
				if d.prefix == "" {
					out.WriteString("xmlns")
				} else {
					out.WriteString("xmlns:" + d.prefix)
				}
				out.WriteString(`="`)
				out.WriteString(escapeC14NAttr(d.uri))
				out.WriteString(`"`)
			}
			for _, a := range attrs {
				out.WriteByte(' ')
				out.WriteString(qname(a.Name))
				out.WriteString(`="`)
				out.WriteString(escapeC14NAttr(a.Value))
				out.WriteString(`"`)
			}
			out.WriteByte('>')
		case xml.EndElement:
			out.WriteString("</")
			out.WriteString(qname(t.Name))
			out.WriteByte('>')
			if len(nsStack) > 0 {
				nsStack = nsStack[:len(nsStack)-1]
			}
		case xml.CharData:
			out.WriteString(escapeC14NText(string(t)))
		case xml.Comment:
			// C14N (sem comentários) remove comentários — descartado de propósito.
		case xml.ProcInst:
			// Declaração XML e demais PIs descartadas — ver limitação documentada.
		case xml.Directive:
			// DOCTYPE descartado — não suportado.
		}
	}
	if !sawRoot {
		return "", fmt.Errorf("no root element")
	}
	return out.String(), nil
}

func escapeC14NAttr(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '"':
			b.WriteString("&quot;")
		case '\t':
			b.WriteString("&#x9;")
		case '\n':
			b.WriteString("&#xA;")
		case '\r':
			b.WriteString("&#xD;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeC14NText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\r':
			b.WriteString("&#xD;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// --- XmlParser/XmlParserFile: árvore de objetos dinâmica ---

// xmlPropName deriva o nome de propriedade AdvPL para um node XML: prefixo
// cReplace + nome do node, com qualquer caractere que não seja letra/dígito/
// underscore substituído por cReplace, tudo em maiúsculas (props do VM só
// são endereçáveis via ":PROP" quando a chave está em maiúsculas — ver
// pkg/vm/vm.go, resolução de propriedade usa strings.ToUpper(propName)).
func xmlPropName(cReplace, tag string) string {
	var b strings.Builder
	b.WriteString(cReplace)
	for _, r := range tag {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteString(cReplace)
		}
	}
	return strings.ToUpper(b.String())
}

// xmlParseDocument faz o parse de raw como XML e monta o objeto "documento"
// retornado por XmlParser/XmlParserFile: um ObjectValue cuja única
// propriedade é o node raiz (nomeada via xmlPropName), espelhando o exemplo
// da TDN (oXml:_PEDIDO:...).
func xmlParseDocument(raw string, cReplace string) (*advplrt.ObjectValue, error) {
	dec := newXMLDecoder(strings.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			rootNode, err := parseXMLNode(dec, start, cReplace)
			if err != nil {
				return nil, err
			}
			doc := advplrt.NewObject("XMLDOCUMENT", nil)
			doc.SetProp(xmlPropName(cReplace, start.Name.Local), rootNode)
			return doc, nil
		}
	}
}

// parseXMLNode constroi recursivamente o ObjectValue de um node XML já
// aberto (start), consumindo tokens de dec até o EndElement correspondente.
// Propriedades sempre presentes: REALNAME (nome original da tag), TYPE
// ("ELEMENT"), TEXT (conteúdo textual direto, sem os filhos, aparado).
// Filhos com nome de tag repetido (>1 ocorrência) são agrupados em
// ARRAYNODES (advplrt.ArrayValue) em vez de props individuais — ver nota de
// interpretação no topo do arquivo.
func parseXMLNode(dec *xml.Decoder, start xml.StartElement, cReplace string) (*advplrt.ObjectValue, error) {
	node := advplrt.NewObject("XMLNODE", nil)
	node.SetProp("REALNAME", advplrt.NewString(start.Name.Local))
	node.SetProp("TYPE", advplrt.NewString("ELEMENT"))

	var textBuf strings.Builder
	type childEntry struct {
		key string
		obj *advplrt.ObjectValue
	}
	var children []childEntry
	counts := map[string]int{}

	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			key := xmlPropName(cReplace, t.Name.Local)
			childNode, err := parseXMLNode(dec, t, cReplace)
			if err != nil {
				return nil, err
			}
			counts[key]++
			children = append(children, childEntry{key, childNode})
		case xml.CharData:
			textBuf.Write(t)
		case xml.EndElement:
			node.SetProp("TEXT", advplrt.NewString(strings.TrimSpace(textBuf.String())))
			var arrayNodes []advplrt.Value
			for _, c := range children {
				if counts[c.key] > 1 {
					arrayNodes = append(arrayNodes, c.obj)
				} else {
					node.SetProp(c.key, c.obj)
				}
			}
			if len(arrayNodes) > 0 {
				node.SetProp("ARRAYNODES", advplrt.NewArray(arrayNodes))
			}
			return node, nil
		}
	}
}

// --- XmlFVldSch: verificador estrutural mínimo (não é um validador XSD completo) ---
//
// Cobre exatamente os dois mecanismos de falha que os próprios exemplos da
// TDN exercitam: elemento obrigatório ausente (xs:sequence/minOccurs) e
// conteúdo que não bate com o tipo primitivo XSD declarado (xs:integer,
// xs:decimal, xs:boolean, xs:date, xs:dateTime; xs:string é sempre válido
// por não ter formato restrito). Não implementa xs:choice/xs:all,
// xs:pattern/xs:enumeration, tipos simples nomeados por restrição, nem
// atributos XML — ver nota de limitação em registerTratamentodeXMLNatives e
// em docs/tdn-known-limitations.md.

// xsdRawNode é uma árvore XML genérica (nome local + atributos por nome
// local, ignorando o namespace "xs:"/"xsd:" do próprio XSD) usada só para
// interpretar a estrutura do arquivo .xsd.
type xsdRawNode struct {
	localName string
	attrs     map[string]string
	children  []*xsdRawNode
}

func parseXsdRaw(data []byte) (*xsdRawNode, error) {
	dec := newXMLDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			return parseXsdRawNode(dec, start)
		}
	}
}

func parseXsdRawNode(dec *xml.Decoder, start xml.StartElement) (*xsdRawNode, error) {
	node := &xsdRawNode{localName: start.Name.Local, attrs: map[string]string{}}
	for _, a := range start.Attr {
		node.attrs[a.Name.Local] = a.Value
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := parseXsdRawNode(dec, t)
			if err != nil {
				return nil, err
			}
			node.children = append(node.children, child)
		case xml.EndElement:
			return node, nil
		}
	}
}

func (n *xsdRawNode) childrenNamed(localName string) []*xsdRawNode {
	var out []*xsdRawNode
	for _, c := range n.children {
		if c.localName == localName {
			out = append(out, c)
		}
	}
	return out
}

// xsdNode é a forma "compilada" e resolvida de um <xs:element>: ou tem
// primType preenchido (folha, checagem de valor) ou children preenchido
// (elemento complexo, checagem de presença + recursão), nunca ambos com
// sentido simultâneo (um elemento tipado com um complexType que por sua vez
// deriva de um tipo simples não é suportado — cai em "sem checagem").
type xsdNode struct {
	primType string // nome local do tipo primitivo XSD (sem prefixo), "" = sem checagem de valor
	children []*xsdChildRef
}

type xsdChildRef struct {
	name      string
	minOccurs int
	node      *xsdNode
}

// xsdPrimitiveTypes são os tipos primitivos XSD com checagem de valor real
// implementada. Qualquer outro nome de tipo (inclusive tipos simples
// nomeados derivados por restrição) é tratado como "sem checagem de valor"
// — documentado, não fingido.
var xsdPrimitiveTypes = map[string]bool{
	"string": true, "integer": true, "int": true, "long": true, "short": true,
	"byte": true, "nonNegativeInteger": true, "positiveInteger": true,
	"negativeInteger": true, "nonPositiveInteger": true, "unsignedInt": true,
	"unsignedLong": true, "unsignedShort": true, "unsignedByte": true,
	"decimal": true, "float": true, "double": true, "boolean": true,
	"date": true, "dateTime": true,
}

// xsdCheckPrimitive valida text contra o tipo primitivo XSD primType. Tipos
// sem formato restrito (string, ou desconhecido) sempre validam.
func xsdCheckPrimitive(primType, text string) bool {
	text = strings.TrimSpace(text)
	switch primType {
	case "integer", "int", "long", "short", "byte", "nonNegativeInteger",
		"positiveInteger", "negativeInteger", "nonPositiveInteger",
		"unsignedInt", "unsignedLong", "unsignedShort", "unsignedByte":
		_, err := strconv.ParseInt(text, 10, 64)
		return err == nil
	case "decimal", "float", "double":
		_, err := strconv.ParseFloat(text, 64)
		return err == nil
	case "boolean":
		return text == "true" || text == "false" || text == "1" || text == "0"
	case "date":
		_, err := time.Parse("2006-01-02", text)
		return err == nil
	case "dateTime":
		_, err := time.Parse(time.RFC3339, text)
		return err == nil
	default:
		// "string" e qualquer tipo não reconhecido: sem checagem de formato.
		return true
	}
}

// localTypeName remove um eventual prefixo de namespace de um valor de
// atributo type="xs:integer" / type="tns:PedidoType" -> "integer" /
// "PedidoType".
func localTypeName(typeAttr string) string {
	if i := strings.LastIndex(typeAttr, ":"); i >= 0 {
		return typeAttr[i+1:]
	}
	return typeAttr
}

// buildXsdNode resolve um <xs:element> (raw) para sua forma compilada,
// resolvendo type="..." contra tipos primitivos conhecidos ou contra
// complexTypeDefs (xs:complexType de topo, por nome). Elementos com type
// desconhecido (não primitivo, não encontrado em complexTypeDefs) ou sem
// type e sem complexType/simpleType inline ficam sem checagem (primType="",
// children=nil) — documentado como gap, não é erro de parsing.
func buildXsdNode(raw *xsdRawNode, complexTypeDefs map[string]*xsdRawNode) *xsdNode {
	if typeAttr, ok := raw.attrs["type"]; ok {
		local := localTypeName(typeAttr)
		if xsdPrimitiveTypes[local] {
			return &xsdNode{primType: local}
		}
		if ct, ok := complexTypeDefs[local]; ok {
			return buildXsdComplexType(ct, complexTypeDefs)
		}
		// type referenciado não resolvido (ex.: tipo simples nomeado por
		// restrição, ou complexType externo) — sem checagem, documentado.
		return &xsdNode{}
	}
	// Sem atributo "type": procura complexType/simpleType inline.
	if inline := raw.childrenNamed("complexType"); len(inline) > 0 {
		return buildXsdComplexType(inline[0], complexTypeDefs)
	}
	if inline := raw.childrenNamed("simpleType"); len(inline) > 0 {
		if restr := inline[0].childrenNamed("restriction"); len(restr) > 0 {
			base := localTypeName(restr[0].attrs["base"])
			if xsdPrimitiveTypes[base] {
				return &xsdNode{primType: base}
			}
		}
		return &xsdNode{}
	}
	// Nem type, nem complexType/simpleType inline: elemento vazio/xs:string
	// implícito — sem checagem de valor específica.
	return &xsdNode{}
}

// buildXsdComplexType resolve um <xs:complexType> (com <xs:sequence> de
// <xs:element>) para a lista de filhos esperados. xs:choice/xs:all não são
// suportados (documentado) — só xs:sequence é interpretado.
func buildXsdComplexType(ct *xsdRawNode, complexTypeDefs map[string]*xsdRawNode) *xsdNode {
	node := &xsdNode{}
	seqs := ct.childrenNamed("sequence")
	if len(seqs) == 0 {
		return node
	}
	for _, elemRaw := range seqs[0].childrenNamed("element") {
		name, ok := elemRaw.attrs["name"]
		if !ok {
			continue
		}
		minOccurs := 1
		if mo, ok := elemRaw.attrs["minOccurs"]; ok {
			if n, err := strconv.Atoi(strings.TrimSpace(mo)); err == nil {
				minOccurs = n
			}
		}
		node.children = append(node.children, &xsdChildRef{
			name:      name,
			minOccurs: minOccurs,
			node:      buildXsdNode(elemRaw, complexTypeDefs),
		})
	}
	return node
}

// xmlInstNode é a árvore genérica da instância XML sendo validada (cXML),
// usada só pelo verificador de schema (independente da árvore de
// advplrt.ObjectValue construída por XmlParser).
type xmlInstNode struct {
	name     string
	text     string
	children map[string][]*xmlInstNode
}

func parseXMLInstance(data []byte) (*xmlInstNode, error) {
	dec := newXMLDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			return parseXMLInstNode(dec, start)
		}
	}
}

func parseXMLInstNode(dec *xml.Decoder, start xml.StartElement) (*xmlInstNode, error) {
	node := &xmlInstNode{name: start.Name.Local, children: map[string][]*xmlInstNode{}}
	var textBuf strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := parseXMLInstNode(dec, t)
			if err != nil {
				return nil, err
			}
			node.children[child.name] = append(node.children[child.name], child)
		case xml.CharData:
			textBuf.Write(t)
		case xml.EndElement:
			node.text = strings.TrimSpace(textBuf.String())
			return node, nil
		}
	}
}

// xsdValidateInstance checa recursivamente inst contra a forma esperada
// node, devolvendo a lista de mensagens de erro encontradas (vazia = válido).
// Mensagens seguem o formato usado pelo próprio exemplo da TDN
// ("Element 'X': 'Y' is not a valid value of the atomic type 'xs:Z'.") —
// úteis para depuração/log, ainda que não possam ser escritas em @cError
// (ver limitação de parâmetros por referência).
func xsdValidateInstance(node *xsdNode, inst *xmlInstNode) []string {
	var errs []string
	if node.primType != "" && !xsdCheckPrimitive(node.primType, inst.text) {
		errs = append(errs, fmt.Sprintf(
			"Element '%s': '%s' is not a valid value of the atomic type 'xs:%s'.",
			inst.name, inst.text, node.primType))
	}
	for _, c := range node.children {
		actual := inst.children[c.name]
		if len(actual) < c.minOccurs {
			errs = append(errs, fmt.Sprintf("Element '%s': missing required child '%s'.", inst.name, c.name))
			continue
		}
		for _, actualChild := range actual {
			errs = append(errs, xsdValidateInstance(c.node, actualChild)...)
		}
	}
	return errs
}

// xsdCheckSchema é o entry point do verificador estrutural: interpreta xsdData
// como um XSD (xsdRawNode -> mapa de xs:complexType de topo por nome ->
// xs:element de topo cujo nome bate com a raiz de xmlData) e valida xmlData
// contra ele. Quando o schema não usa a forma reconhecida (sem
// xs:element de topo cujo nome bata com a raiz do XML, por exemplo), não há
// como aplicar a checagem estrutural — devolve true (mesma postura de
// "apenas bem-formado" da versão anterior desta função, documentada: um
// schema fora do subconjunto reconhecido não reprova o XML só por isso).
func xsdCheckSchema(xmlData, xsdData []byte) bool {
	xsdRoot, err := parseXsdRaw(xsdData)
	if err != nil || xsdRoot.localName != "schema" {
		return true
	}
	inst, err := parseXMLInstance(xmlData)
	if err != nil {
		return true
	}

	complexTypeDefs := map[string]*xsdRawNode{}
	for _, ct := range xsdRoot.childrenNamed("complexType") {
		if name, ok := ct.attrs["name"]; ok {
			complexTypeDefs[name] = ct
		}
	}

	for _, elemRaw := range xsdRoot.childrenNamed("element") {
		if elemRaw.attrs["name"] != inst.name {
			continue
		}
		root := buildXsdNode(elemRaw, complexTypeDefs)
		return len(xsdValidateInstance(root, inst)) == 0
	}
	// Nenhum <xs:element> de topo com o nome da raiz do XML: forma do
	// schema não reconhecida por este verificador mínimo — sem checagem.
	return true
}
