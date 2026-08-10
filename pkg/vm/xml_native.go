package vm

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

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
// XML Schema (XSD) — go.mod não lista nenhuma dependência de
// schema/xmlsec/c14n (conferido). Implementar um validador XSD real e
// completo é um projeto por si só, fora do escopo desta task. O que É
// implementado de verdade: leitura real dos dois arquivos (cXML, cXSD) do
// disco e checagem de boa-formação XML de ambos. O que NÃO é feito (gap
// documentado, não fingido): nenhuma checagem de conformidade com os
// constraints do XSD (tipos como xs:integer, elementos obrigatórios,
// enumerações, cardinalidade). Isso significa que, ao contrário do segundo
// exemplo da própria TDN (invalid.xml com Quantidade='ABC' deveria retornar
// .F.), esta implementação retornaria .T. para um XML bem-formado mesmo
// violando o schema — a função só prova boa-formação + existência dos
// arquivos, não conformidade de schema. Documentado explicitamente aqui e
// em docs/tdn-known-limitations.md; não deve ser usada como XSD validator
// real.
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
	// conforme TDN ("Only server path are allowed on XmlC14NFile").
	natives["XMLC14NFILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFile := strings.Trim(getArgString(args, 0, ""), " ")
		if cFile == "" {
			return advplrt.NewString(""), nil
		}
		if smartClientPathPattern.MatchString(cFile) {
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
	// Valida um arquivo XML contra um XSD. AdvPP não tem validador de XML
	// Schema real (ver nota de limitação acima) — checa apenas: os dois
	// arquivos existem e são legíveis, e cXML é XML bem-formado (o próprio
	// cXSD também precisa ser XML bem-formado, já que XSD é um dialeto XML).
	// NÃO valida constraints de schema (tipos, obrigatoriedade, enumeração).
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
		return advplrt.True, nil
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
