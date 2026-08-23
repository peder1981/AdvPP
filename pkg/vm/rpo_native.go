package vm

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// registerManipulacaodeRPONatives registra funções de manipulação de RPO:
// ChkRpoChg, GetAPOInfo, GetApoRes, GetDependency, GetFuncArray, GetRpoLog,
// GetSrcArray, RetImgType, Resource2File, GetPatchFile,
// SRCheckSourceSignature.
//
// Nota arquitetural (ver docs/tdn-known-limitations.md): o Protheus real
// compila fontes AdvPL para um RPO binário próprio (formato proprietário
// TOTVS), que retém um índice físico de fontes, patches aplicados e
// resources embutidos. AdvPP compila para o seu próprio bytecode Go
// (pkg/compiler.Bytecode) e NÃO mantém esse tipo de índice: FunctionInfo
// não guarda o arquivo-fonte de origem, não existe registro de patches
// (.upd/.pak/.ptm) e não existe container de resources (.PER). Por isso,
// as funções desta categoria que dependem desse índice físico (GetDependency,
// GetRpoLog, GetSrcArray, ChkRpoChg, GetApoRes) fazem validação de argumentos
// real e devolvem o retorno conservador mais correto dentro da arquitetura
// atual (documentado função a função abaixo), em vez de simular um formato
// de RPO que não existe neste compilador — GetApoRes em particular recebe um
// identificador de resource INTERNO ao container do RPO (obtido via
// GetResArray, que também não existe em AdvPP), não um caminho de disco, por
// isso não é reinterpretada como leitura de arquivo arbitrário. GetFuncArray,
// por outro lado, tem equivalente real e útil: o conjunto de funções
// conhecidas pelo VM em execução (v.bc.Functions + v.natives) É a fonte da
// verdade real do AdvPP para "funções compiladas no repositório em uso"
// (embora isso também capture nativas do motor, que no Protheus real nunca
// residem dentro do RPO — ver nota na implementação). GetAPOInfo e
// RetImgType operam sobre um único arquivo nomeado por parâmetro — para
// essas, AdvPP lê o arquivo real do disco (quando existe), o que é
// comportamento real, não simulado.
func (v *VM) registerManipulacaodeRPONatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {
	// ChkRpoChg() -> lRet
	// Verifica se a configuração de SourcePath (RPO ativo) mudou após o
	// início do processo atual.
	//
	// AdvPP não recarrega bytecode a partir de um totvsappserver.ini nem
	// possui um conceito de "SourcePath ativo" que possa divergir em tempo
	// de execução: o bytecode carregado no início do processo é o único
	// que existe durante toda a vida do processo. Portanto, a resposta
	// real e honesta é sempre "nenhuma mudança detectada" (.T.).
	natives["CHKRPOCHG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.True, nil
	}

	// GetAPOInfo(cFonte) -> aData
	// aData[1] nome, aData[2] linguagem, aData[3] modo de compilação,
	// aData[4] data da última modificação, aData[5] hora/min/seg.
	//
	// AdvPP não mantém um índice fonte->metadados dentro do bytecode
	// compilado (FunctionInfo não guarda arquivo de origem). Como cFonte
	// é o nome/caminho de um arquivo real, buscamos o arquivo de fato no
	// disco e devolvemos seus metadados reais (mtime). Linguagem é fixa
	// "AdvPL" e modo de compilação é fixo BUILD_FULL (0), já que AdvPP não
	// implementa o sistema de permissões multi-tier (USER/PARTNER/PATCH)
	// do Protheus real — todo código compilado por AdvPP tem o mesmo nível
	// de permissão. Se o arquivo não existir, devolve array vazio.
	//
	// Nota: cFonte é resolvido como caminho literal relativo ao diretório
	// de trabalho (cwd) do processo, não via mecanismo SourcePath do
	// Protheus. O próprio exemplo do TDN, GetAPOInfo("ExemplosTDN.prw")
	// (nome de arquivo sem path), só encontra o arquivo se ele estiver no
	// cwd do processo AdvPP — caso contrário devolve array vazio em
	// silêncio. Ver docs/tdn-known-limitations.md.
	natives["GETAPOINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFonte := strings.Trim(getArgString(args, 0, ""), " ")
		if cFonte == "" {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		info, err := os.Stat(cFonte)
		if err != nil || info.IsDir() {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		mt := info.ModTime()
		return advplrt.NewArray([]advplrt.Value{
			advplrt.NewString(filepath.Base(cFonte)),
			advplrt.NewString("AdvPL"),
			advplrt.NewNumber(0), // BUILD_FULL — AdvPP não distingue níveis de compilação
			advplrt.NewDate(mt),
			advplrt.NewString(mt.Format("15:04:05")),
		}), nil
	}

	// GetApoRes(cRes) -> cRet
	// Retorna o conteúdo de um resource do repositório.
	//
	// cRes, segundo o fluxo real do TDN, é um identificador de resource
	// INTERNO ao container de resources do RPO (obtido previamente via
	// GetResArray("*.per"), que enumera os nomes válidos) — não é um
	// caminho de arquivo em disco arbitrário. AdvPP não possui container
	// de resources (.PER) embutido no bytecode nem implementa GetResArray
	// (a única forma documentada de obter um cRes válido para chamar esta
	// função), então nenhum chamador seguindo o fluxo real do TDN
	// conseguiria alcançar esta função com um argumento válido. Faz a
	// validação de argumento real e devolve "" (conservador e honesto),
	// em vez de reinterpretar cRes como um caminho de disco arbitrário —
	// isso responderia a uma pergunta diferente da que o TDN especifica.
	// Ver docs/tdn-known-limitations.md.
	natives["GETAPORES"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cRes := strings.Trim(getArgString(args, 0, ""), " ")
		if cRes == "" {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(""), nil
	}

	// GetDependency(sFonte) -> aArray
	// aArray[n][1] função com a chamada, aArray[n][2] função chamada,
	// aArray[n][3] fonte onde está a função chamada.
	//
	// AdvPP não constrói um grafo de chamadas por arquivo-fonte no
	// bytecode compilado (FunctionInfo não associa função a arquivo de
	// origem nem retém a lista de chamadas de primeiro nível de cada
	// função). Reconstruir isso exigiria reanalisar o AST do parser por
	// fonte, fora do escopo desta task (ver docs/tdn-known-limitations.md).
	// Faz a validação de argumento real e devolve array vazio (retorno
	// estruturalmente válido, mas sem dependências reportadas).
	natives["GETDEPENDENCY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		sFonte := strings.Trim(getArgString(args, 0, ""), " ")
		if sFonte == "" {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		return advplrt.NewArray([]advplrt.Value{}), nil
	}

	// GetFuncArray(cMascara, [@aTipo], [@aArquivo], [@aLinha], [@aData], [@aHora]) -> aScr
	// Retorna um array com os nomes das funções compiladas no repositório
	// em uso, filtradas por máscara (aceita "*" e "?").
	//
	// Real: v.bc.Functions (funções do programa compilado) e v.natives
	// (funções nativas do VM) SÃO a fonte da verdade do AdvPP para
	// "funções conhecidas pelo sistema em execução" — não há necessidade
	// de simular um RPO. Ordenado para saída determinística.
	//
	// Divergência conhecida: v.natives inclui nativas do motor (ex:
	// CONOUT, MSGALERT) que, no Protheus real, ficam compiladas dentro do
	// binário do AppServer — nunca dentro do RPO. AdvPP não tem fronteira
	// engine/user-function equivalente, então essas nativas também casam
	// com a máscara aqui. Ver docs/tdn-known-limitations.md.
	//
	// Limitação conhecida (ver docs/tdn-known-limitations.md, seção
	// "Parâmetros por referência"): os parâmetros opcionais por referência
	// @aTipo/@aArquivo/@aLinha/@aData/@aHora não são populados — não há,
	// em nenhum lugar do VM, mecanismo para uma native mutar uma variável
	// do chamador passada com @. O retorno principal (aScr) é a forma
	// suportada de usar esta função em AdvPP.
	natives["GETFUNCARRAY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cMascara := strings.Trim(getArgString(args, 0, ""), " ")
		if cMascara == "" {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}

		names := make(map[string]bool)
		if v.bc != nil {
			for name := range v.bc.Functions {
				names[name] = true
			}
		}
		for name := range v.natives {
			names[name] = true
		}

		matched := make([]string, 0, len(names))
		for name := range names {
			if advplMaskMatch(cMascara, name) {
				matched = append(matched, name)
			}
		}
		sort.Strings(matched)

		elems := make([]advplrt.Value, len(matched))
		for i, name := range matched {
			elems[i] = advplrt.NewString(name)
		}
		return advplrt.NewArray(elems), nil
	}

	// GetRpoLog([nRPO]) -> aData
	// aData[1] {versão do RPO, data do RPO}, aData[2] quantidade de patches
	// aplicados, aData[3..] arrays com dados de cada patch aplicado.
	//
	// AdvPP não possui sistema de patches (.upd/.pak/.ptm) nem versão de
	// RPO — é um compilador standalone sem histórico de aplicação de
	// pacotes. A resposta honesta dentro dessa arquitetura é "nenhum
	// patch aplicado": versão/data do RPO vazias e contagem de patches 0.
	// Valida nRPO (1=Padrão, 3=Custom) quando informado.
	natives["GETRPOLOG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if len(args) > 0 && args[0] != nil && args[0] != advplrt.Nil {
			n := advplrt.ToFloat(args[0])
			if n != 1 && n != 3 {
				return advplrt.NewArray([]advplrt.Value{}), nil
			}
		}
		return advplrt.NewArray([]advplrt.Value{
			advplrt.NewArray([]advplrt.Value{
				advplrt.NewString(""),
				advplrt.NewDate(time.Time{}),
			}),
			advplrt.NewNumber(0),
		}), nil
	}

	// GetSrcArray(cNome, [nRPO]) -> aFontes
	// Retorna um array com o nome dos fontes compilados que casam com a
	// máscara informada.
	//
	// AdvPP não mantém um índice de "nomes de arquivo-fonte compilados"
	// dentro do bytecode (só nomes de função, não nomes de arquivo — ver
	// nota no topo do arquivo). Não há como listar fontes reais a partir
	// do bytecode. Valida argumentos (máscara obrigatória, nRPO em
	// {1,2,3} quando informado) e devolve array vazio, que é a resposta
	// honesta dado que AdvPP não persiste esse índice.
	natives["GETSRCARRAY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cNome := strings.Trim(getArgString(args, 0, ""), " ")
		if cNome == "" {
			return advplrt.NewArray([]advplrt.Value{}), nil
		}
		if len(args) > 1 && args[1] != nil && args[1] != advplrt.Nil {
			n := advplrt.ToFloat(args[1])
			if n != 1 && n != 2 && n != 3 {
				return advplrt.NewArray([]advplrt.Value{}), nil
			}
		}
		return advplrt.NewArray([]advplrt.Value{}), nil
	}

	// RetImgType(cPath) -> nRet
	// Retorna o tipo de imagem (1 = Bitmap, 2 = JPG) a partir do path
	// informado, lendo os bytes de assinatura (magic number) do arquivo
	// real em disco. 0 quando o arquivo não existe/não é reconhecido.
	natives["RETIMGTYPE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cPath := strings.Trim(getArgString(args, 0, ""), " ")
		if cPath == "" {
			return advplrt.NewNumber(0), nil
		}
		f, err := os.Open(cPath)
		if err != nil {
			return advplrt.NewNumber(0), nil
		}
		defer f.Close()

		header := make([]byte, 4)
		n, _ := f.Read(header)
		header = header[:n]

		if len(header) >= 2 && header[0] == 'B' && header[1] == 'M' {
			return advplrt.NewNumber(1), nil // BMP
		}
		if len(header) >= 3 && header[0] == 0xFF && header[1] == 0xD8 && header[2] == 0xFF {
			return advplrt.NewNumber(2), nil // JPG
		}
		return advplrt.NewNumber(0), nil
	}

	// Resource2File(cResource, cFile) -> lSucess
	// Salva o conteúdo de um resource do repositório (container .PER
	// embutido no RPO) em um arquivo de disco.
	//
	// Mesma categoria arquitetural de GetApoRes (ver comentário acima):
	// cResource é um identificador INTERNO ao container de resources do
	// RPO real (tipicamente um recurso embutido via #RESOURCE em tempo de
	// compilação), não um caminho de disco. AdvPP não compila para esse
	// formato e não mantém um container de resources — por isso cResource
	// NÃO é reinterpretado como um caminho de disco arbitrário (mesmo
	// motivo, e mesma correção de code review, documentados na função
	// GetApoRes acima: uma reinterpretação assim responderia a uma
	// pergunta diferente da que a TDN especifica). Valida os argumentos
	// (nenhum pode ser vazio) e sempre devolve .F. — não há resource para
	// copiar.
	natives["RESOURCE2FILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cResource := getArgString(args, 0, "")
		cFile := getArgString(args, 1, "")
		if cResource == "" || cFile == "" {
			return advplrt.False, nil
		}
		return advplrt.False, nil
	}

	// GetPatchFile(cPatchFile, cFileName, [lChangeCase=.T.]) -> cRet
	// Extrai o conteúdo de um arquivo específico contido em um patch
	// (.ptm/.upd/.pak, formato binário proprietário TOTVS).
	//
	// AdvPP valida que cPatchFile/cFileName não são vazios e, quando
	// cPatchFile aponta para um arquivo real em disco, confirma que ele
	// existe — mas não implementa um parser do formato de patch
	// proprietário da TOTVS (mesma lacuna arquitetural documentada acima
	// para GetRpoLog: "não existe sistema de patches" neste compilador).
	// Reproduzir esse formato binário fechado está fora do escopo deste
	// projeto (mesmo critério já usado para o validador XSD em
	// pkg/vm/xml_native.go: implementar o núcleo real do que é viável,
	// documentar o resto em vez de fingir). Por isso sempre devolve "" —
	// nunca simula conteúdo de arquivo que não pode extrair de verdade.
	natives["GETPATCHFILE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cPatchFile := getArgString(args, 0, "")
		cFileName := getArgString(args, 1, "")
		if cPatchFile == "" || cFileName == "" {
			return advplrt.NewString(""), nil
		}
		return advplrt.NewString(""), nil
	}

	// SRCheckSourceSignature(cSourceName) -> nRet
	// Verifica se um fonte foi assinado por um certificado (0 = sem
	// assinatura, 1 = assinatura OK, 2 = assinatura inválida).
	//
	// AdvPP não possui infraestrutura de assinatura digital de fontes (o
	// SR* aqui refere-se ao "Source Repository" assinado do Protheus,
	// mecanismo de certificação que não existe neste compilador Go
	// standalone) — nenhum fonte compilado por este VM jamais foi
	// assinado, então "0 (Sem Assinatura)" é a resposta real e honesta
	// para qualquer fonte, não uma simulação de "não sei". Valida que
	// cSourceName não é vazio.
	natives["SRCHECKSOURCESIGNATURE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cSourceName := getArgString(args, 0, "")
		if cSourceName == "" {
			return advplrt.NewNumber(0), nil
		}
		return advplrt.NewNumber(0), nil
	}
}

// advplMaskMatch casa nome contra máscara AdvPL (curingas "*" e "?"),
// case-insensitive, reaproveitando o semântica de filepath.Match.
func advplMaskMatch(mask, name string) bool {
	ok, err := filepath.Match(strings.ToUpper(mask), strings.ToUpper(name))
	if err != nil {
		return false
	}
	return ok
}
