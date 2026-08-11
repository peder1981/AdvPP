package vm

// Funções de Ambiente — TDN: Functions/Ambiente/Funcoes-genericas.
//
// Implementação das natives que descrevem o ambiente onde a VM está rodando
// (hostname, SO, arquitetura, memória, UUID, métricas, etc). O AdvPP não é um
// Application Server Protheus nem um SmartClient, então toda informação que
// no Protheus real vem de dentro do appserver/smartclient é extraída aqui do
// ambiente real do processo Go (os/runtime/net) ou devolvida com um valor
// sensato documentado no próprio native.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// ambienteProcessStart guarda o instante em que o processo Go iniciou — usado
// por MetricsRead (métrica startdate), GetUserInfoArray (tempo ativo) e
// GetSrvGlbInfo.
var ambienteProcessStart = time.Now()

// ambienteUUIDSeqCounter é o contador monotônico global usado por
// UUIDRandomSeq. Inicializado com o relógio para reduzir colisão entre
// processos distintos; o incremento é atômico (seguro para chamadas
// concorrentes de threads/jobs).
var ambienteUUIDSeqCounter uint64

func init() {
	atomic.StoreUint64(&ambienteUUIDSeqCounter, uint64(time.Now().UnixNano()))
}

// Estado global do SetKSysLog/DelKSysLog: um conjunto de identificadores
// "[chave valor]" que no Protheus real são anexados a toda mensagem de syslog
// (ConOut/LogMsg). No AdvPP o registro é persistido num arquivo de log do
// sistema (KSYSLOG ou os.TempDir()/advpp_ksyslog.log) — a tag em si fica
// disponível via syslogTagString() para um eventual hook de ConOut/LogMsg,
// que não foi alterado (natives.go é intocado).
var (
	ksyslogMu   sync.Mutex
	ksyslogTags = make(map[string]string)
)

func ksyslogFilePath() string {
	if p := os.Getenv("KSYSLOG"); p != "" {
		return p
	}
	return filepath.Join(os.TempDir(), "advpp_ksyslog.log")
}

// syslogTagString devolve os identificadores ativos como " [chave valor]"...
// Não é chamada por ConOut/LogMsg hoje (o natives.go não foi alterado); fica
// exposta para integração futura.
func syslogTagString() string {
	ksyslogMu.Lock()
	defer ksyslogMu.Unlock()
	parts := make([]string, 0, len(ksyslogTags))
	for k, v := range ksyslogTags {
		parts = append(parts, fmt.Sprintf("[%s %s]", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

// registerAmbienteNatives registra as funções de Ambiente:
// GetCodePage, GetComputerName, GetEnvServer, GetHardwareId, GetImpWindows,
// GetLinesProg, GetPowerSC, GetPublicIP, GetClientIP, GetRemoteIniName,
// GetRmtDate, GetRmtInfo, GetRmtTime, GetRmtVersion, GetServerIP,
// GetServerType, GetSrvArch, GetSrvGlbInfo, GetSrvInfo, GetSrvIniName,
// GetSrvMemInfo, GetSrvNickName, GetSrvOSInfo, GetTempPath, GetUserInfoArray,
// GetWebAgentInfo, IsRmt64, IsSrvUnix, MetricsName, MetricsRead,
// RemoteXVersion, SerialNumber, SetKSysLog, DelKSysLog, ShowInfMem,
// UUIDRandom, UUIDRandomSeq.
func (v *VM) registerAmbienteNatives(natives map[string]func(args []advplrt.Value) (advplrt.Value, error)) {

	// GetCodePage() -> cRet
	// Retorna o encode definido no .ini do Application Server. Sem .ini no
	// AdvPP, devolve o encode padrão documentado pelo TDN: CP1252.
	natives["GETCODEPAGE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString("CP1252"), nil
	}

	// GetComputerName() -> cRet
	// Nome da máquina (hostname) onde o cliente está sendo executado.
	natives["GETCOMPUTERNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if host, err := os.Hostname(); err == nil {
			return advplrt.NewString(host), nil
		}
		return advplrt.Nil, nil
	}

	// GetEnvServer() -> cRet
	// Nome do ambiente (environment) em execução no Application Server.
	// No AdvPP o nome do ambiente é o da variável de ambiente
	// PROTHEUS_ENVIRONMENT (ou GETENVSERVER); sem elas, "Default".
	natives["GETENVSERVER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		for _, k := range []string{"PROTHEUS_ENVIRONMENT", "GETENVSERVER"} {
			if v := os.Getenv(k); v != "" {
				return advplrt.NewString(v), nil
			}
		}
		return advplrt.NewString("Default"), nil
	}

	// GetHardwareId() -> cID
	// No Protheus real retorna o serial do drive (Windows) ou "FFFF-FFFF"
	// em erro/não-Windows. O AdvPP roda em qualquer SO e deriva um ID
	// estável do host (hash de hostname + machine-id) no mesmo formato
	// "XXXX-XXXX" — ver SerialNumber.
	natives["GETHARDWAREID"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(stableHostID()), nil
	}

	// GetImpWindows([lDirect]) -> aRet
	// Array com os nomes das impressoras disponíveis (1ª é a padrão).
	// No Linux tenta "lpstat -p"; sem CUPS ou em Windows retorna {}.
	natives["GETIMPWINDOWS"] = func(args []advplrt.Value) (advplrt.Value, error) {
		printers := listPrinters()
		elems := make([]advplrt.Value, len(printers))
		for i, p := range printers {
			elems[i] = advplrt.NewString(p)
		}
		return advplrt.NewArray(elems), nil
	}

	// GetLinesProg([cFile]) -> nLines
	// Número de linhas executáveis do fonte. O AdvPP não mantém o índice
	// fonte->bytecode de um RPO, então: sem argumento retorna 0; com um
	// caminho de arquivo real, conta as linhas do arquivo em disco.
	natives["GETLINESPROG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cFile := getArgString(args, 0, "")
		if cFile == "" {
			return advplrt.NewNumber(0), nil
		}
		if n, err := countLines(cFile); err == nil {
			return advplrt.NewNumber(float64(n)), nil
		}
		return advplrt.NewNumber(0), nil
	}

	// GetPowerSC() -> aPSInfo
	// Array de arrays [nome, tipo, cpu] com o plano de energia por CPU.
	// Linux: lê scaling_governor de cada CPU (performance=5, powersave=6,
	// userspace=7, ondemand=8, conservative=9, schedutil=10; senão
	// "Unknown Scheme"=0). Outros SOs: array vazio.
	natives["GETPOWERSC"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewArray(powerSchemes()), nil
	}

	// GetPublicIP([@bHasIP]) -> cIP
	// IP público. SEM chamadas HTTP externas: apenas os endereços das
	// interfaces locais; se nenhuma tiver IP público (rede privada), retorna
	// "". O parâmetro por referência @bHasIP não é gravável nesta VM (mesma
	// limitação das demais natives com byref — ver execucaoprocessos_native).
	natives["GETPUBLICIP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		for _, ip := range localInterfaceIPs() {
			if isPublicIP(ip) {
				return advplrt.NewString(ip.String()), nil
			}
		}
		return advplrt.NewString(""), nil
	}

	// GetClientIP([lClientSide]) -> cRet
	// IP em uso pelo SmartClient para conectar no servidor. No AdvPP não há
	// conexão SmartClient; devolve o primeiro IP não-loopback das interfaces.
	natives["GETCLIENTIP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(firstNonLoopbackIP()), nil
	}

	// GetRemoteIniName() -> cRet
	// Caminho do arquivo de configuração do SmartClient. Sem SmartClient no
	// AdvPP, usa a variável SMARTCLIENT_INI ou o nome "smartclient.ini".
	natives["GETREMOTEININAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if p := os.Getenv("SMARTCLIENT_INI"); p != "" {
			return advplrt.NewString(p), nil
		}
		return advplrt.NewString("smartclient.ini"), nil
	}

	// GetRmtDate() -> dRet
	// Data atual do sistema onde o cliente roda.
	natives["GETRMTDATE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewDate(time.Now()), nil
	}

	// GetRmtInfo() -> aRet
	// Array com as definições do computador do cliente (13 posições).
	natives["GETRMTINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewArray(remoteInfoArray()), nil
	}

	// GetRmtTime() -> cRet
	// Hora atual do sistema onde o cliente roda.
	natives["GETRMTTIME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(time.Now().Format("15:04:05")), nil
	}

	// GetRmtVersion() -> cRet
	// Versão do SmartClient. O AdvPP não tem SmartClient; devolve a versão
	// padrão do runtime (placeholder documentado).
	natives["GETRMTVERSION"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(advppVersion), nil
	}

	// GetServerIP([lGetAllAddress]) -> cRet | aAddrs
	// IP do servidor. lGetAllAddress=.T. devolve array de arrays
	// [tipo(s), socketType, protocolo, IP] com todos os endereços.
	natives["GETSERVERIP"] = func(args []advplrt.Value) (advplrt.Value, error) {
		lAll := advplrt.ToBool(getArg(args, 0))
		if !lAll {
			return advplrt.NewString(firstNonLoopbackIP()), nil
		}
		var rows []advplrt.Value
		for _, ip := range allInterfaceIPs() {
			cType := "IPv4"
			if ip.To4() == nil {
				cType = "IPv6"
			}
			rows = append(rows, advplrt.NewArray([]advplrt.Value{
				advplrt.NewString(cType),       // 1: tipo do IP
				advplrt.NewNumber(0),           // 2: socketType (0 - Unspecified)
				advplrt.NewNumber(0),           // 3: protocolo (0 - TCP)
				advplrt.NewString(ip.String()), // 4: endereço IP
			}))
		}
		return advplrt.NewArray(rows), nil
	}

	// GetServerType() -> nRet
	// Tipo de execução do Application Server: 0=None, 1=Console, 2=ISAPI,
	// 3=FAT. O AdvPP executa em modo texto: 1 (Console).
	natives["GETSERVERTYPE"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewNumber(1), nil
	}

	// GetSrvArch() -> cRet
	// Arquitetura do processador no formato linux base.
	natives["GETSRVARCH"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(srvArchName()), nil
	}

	// GetSrvGlbInfo() -> cGlbInfo
	// String com resumo do status do serviço. Construída com dados reais do
	// runtime Go (goroutines, memória) no mesmo formato seccionado do TDN.
	natives["GETSRVGLBINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(srvGlbInfo()), nil
	}

	// GetSrvInfo() -> aSrvInfo
	// Array (13 posições) com as definições do servidor onde o Application
	// Server foi instanciado.
	natives["GETSRVINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewArray(srvInfoArray()), nil
	}

	// GetSrvIniName() -> cRet
	// Nome do arquivo de configuração do Application Server. Sem appserver
	// real, usa a variável PROTHEUS_INI ou "appserver.ini".
	natives["GETSRVININAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if p := os.Getenv("PROTHEUS_INI"); p != "" {
			return advplrt.NewString(p), nil
		}
		return advplrt.NewString("appserver.ini"), nil
	}

	// GetSrvMemInfo() -> cMemInfo
	// String com o resumo de memória da máquina (Physical memory / Paging).
	natives["GETSRVMEMINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(memSummary()), nil
	}

	// GetSrvNickName() -> cRet
	// Apelido do Application Server. Sem servidor real, devolve "Local".
	natives["GETSRVNICKNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString("Local"), nil
	}

	// GetSrvOSInfo() -> cSrvOsInfo
	// Informações do Sistema Operacional onde o servidor roda.
	natives["GETSRVOSINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(srvOSInfo()), nil
	}

	// GetTempPath([lLocal], [lWeb]) -> cRet
	// Caminho da pasta temporária do sistema. Sem SmartClient/WebAgent no
	// AdvPP, sempre devolve os.TempDir() (lLocal/lWeb são aceitos e
	// ignorados).
	natives["GETTEMPPATH"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(os.TempDir()), nil
	}

	// GetUserInfoArray([lShowMoreInfo]) -> aRet
	// Array multidimensional com os processos em execução. O AdvPP devolve
	// uma única linha com o processo atual da VM.
	natives["GETUSERINFOARRAY"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewArray([]advplrt.Value{advplrt.NewArray(userInfoRow())}), nil
	}

	// GetWebAgentInfo() -> aRet
	// Array com as definições do WebAgent. Função exclusiva do SmartClient
	// WebApp; fora dele retorna array vazio.
	natives["GETWEBAGENTINFO"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewArray([]advplrt.Value{}), nil
	}

	// IsRmt64() -> lRet
	// Verdadeiro se o binário do cliente for 64-bit. Aqui, a arquitetura da
	// própria VM.
	natives["ISRMT64"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(runtimeArchIs64()), nil
	}

	// IsSrvUnix() -> lRet
	// Verdadeiro se o servidor roda em Unix/Linux.
	natives["ISSRVUNIX"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewBool(runtime.GOOS != "windows"), nil
	}

	// MetricsName([WithVersion]) -> cRet
	// Objeto JSON com os nomes das métricas disponíveis (e versão da API
	// quando WithVersion=.T.).
	natives["METRICSNAME"] = func(args []advplrt.Value) (advplrt.Value, error) {
		names := metricNames()
		var out map[string]interface{}
		if advplrt.ToBool(getArg(args, 0)) {
			out = map[string]interface{}{"version": 0, "names": names}
		} else {
			out = map[string]interface{}{"names": names}
		}
		b, err := json.Marshal(out)
		if err != nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(string(b)), nil
	}

	// MetricsRead([Metric_Name]) -> cRet
	// Objeto JSON com as métricas coletadas. Filtro por array de nomes;
	// nomes inválidos entram com a propriedade error.
	natives["METRICSREAD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		var filter []string
		if a, ok := getArg(args, 0).(*advplrt.ArrayValue); ok && a != nil {
			for _, el := range a.Elements {
				filter = append(filter, advplrt.ToString(el))
			}
		}
		metrics := collectMetrics(filter)
		wrapper := []interface{}{map[string]interface{}{"version": 0, "metrics": metrics}}
		b, err := json.Marshal(wrapper)
		if err != nil {
			return advplrt.Nil, nil
		}
		return advplrt.NewString(string(b)), nil
	}

	// RemoteXVersion() -> cVersion
	// Build do Smart Client ActiveX no formato "8,YY,MMDD,0" (zero à
	// esquerda de ano/mês omitido). Sem ActiveX real, a "build" é derivada
	// da data corrente.
	natives["REMOTEXVERSION"] = func(args []advplrt.Value) (advplrt.Value, error) {
		now := time.Now()
		yy := now.Year() % 100
		mm := int(now.Month())
		dd := now.Day()
		return advplrt.NewString(fmt.Sprintf("8,%d,%d%d,0", yy, mm, dd)), nil
	}

	// SerialNumber([cDrive]) -> cID
	// No Protheus real, serial de um drive Windows (ex.: "9031-1ED5"); fora
	// do Windows retorna "0000-0000". O AdvPP roda em qualquer SO e deriva
	// um ID estável do host (hash de hostname + machine-id), estável entre
	// chamadas, no mesmo formato "XXXX-XXXX".
	natives["SERIALNUMBER"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(stableHostID()), nil
	}

	// SetKSysLog(ckey, cValor) -> Nil
	// Adiciona um identificador "[chave valor]" às mensagens de syslog. No
	// AdvPP registra a tag no estado global e anexa a linha ao arquivo de
	// log do sistema (KSYSLOG ou <temp>/advpp_ksyslog.log).
	natives["SETKSYSLOG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		key := getArgString(args, 0, "")
		val := getArgString(args, 1, "")
		if key == "" {
			return advplrt.Nil, nil
		}
		ksyslogMu.Lock()
		ksyslogTags[key] = val
		path := ksyslogFilePath()
		ksyslogMu.Unlock()
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			fmt.Fprintf(f, "[%s %s]\n", key, val)
			f.Close()
		}
		return advplrt.Nil, nil
	}

	// DelKSysLog(ckey) -> Nil
	// Remove um identificador previamente adicionado por SetKSysLog. Quando
	// não restam tags, remove o arquivo de log do sistema.
	natives["DELKSYSLOG"] = func(args []advplrt.Value) (advplrt.Value, error) {
		key := getArgString(args, 0, "")
		ksyslogMu.Lock()
		delete(ksyslogTags, key)
		if len(ksyslogTags) == 0 {
			os.Remove(ksyslogFilePath())
		}
		ksyslogMu.Unlock()
		return advplrt.Nil, nil
	}

	// ShowInfMem([cTexto], [aInfo]) -> lRet
	// Exibe contadores de memória (pools) e, se aInfo (array bidimensional)
	// for passado por referência, preenche cada elemento com [kb, count].
	natives["SHOWINFMEM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		cTexto := getArgString(args, 0, "")
		if cTexto != "" {
			fmt.Fprintf(os.Stderr, "[ShowInfMem] %s\n", cTexto)
		}
		if a, ok := getArg(args, 1).(*advplrt.ArrayValue); ok && a != nil {
			fillMemPoolArray(a)
		}
		return advplrt.True, nil
	}

	// UUIDRandom() -> cUUID
	// UUID v4 gerado com crypto/rand no formato 8-4-4-4-12 (lowercase).
	natives["UUIDRANDOM"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if u, ok := newUUIDv4(); ok {
			return advplrt.NewString(u), nil
		}
		return advplrt.Nil, nil
	}

	// UUIDRandomSeq() -> cUUID
	// UUID sequencial: contador monotônico global (atomic) nos 8 bytes mais
	// significativos + 8 bytes aleatórios; formato 8-4-4-4-12 (lowercase).
	natives["UUIDRANDOMSEQ"] = func(args []advplrt.Value) (advplrt.Value, error) {
		if u, ok := newUUIDSeq(); ok {
			return advplrt.NewString(u), nil
		}
		return advplrt.Nil, nil
	}

	// =========================================================================
	// CmpBuildStr(cLeft, cRight) -> nEq
	//   Compara duas strings em formato nnn.nnn.nnn.nnn considerando os 4
	//   primeiros blocos numéricos. Retorna 0 (iguais), 1 (cLeft maior) ou
	//   -1 (cLeft menor). Blocos ausentes/não numéricos valem 0 (spec).
	// =========================================================================
	natives["CMPBUILDSTR"] = func(args []advplrt.Value) (advplrt.Value, error) {
		a := parseBuildBlocks(getArgString(args, 0, ""))
		b := parseBuildBlocks(getArgString(args, 1, ""))
		for i := 0; i < 4; i++ {
			if a[i] < b[i] {
				return advplrt.NewNumber(-1), nil
			}
			if a[i] > b[i] {
				return advplrt.NewNumber(1), nil
			}
		}
		return advplrt.NewNumber(0), nil
	}

	// =========================================================================
	// GetBuild([lType]) -> cBuild
	//   String com informações da build em uso. lType=.T. indica SmartClient
	//   (.T.) ou Application Server (.F., padrão). O AdvPP não roda
	//   SmartClient real; devolve a versão do runtime (placeholder
	//   documentado, igual a GetRmtVersion).
	// =========================================================================
	natives["GETBUILD"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(advppVersion), nil
	}

	// =========================================================================
	// GetEndPoint([@bBroker]) -> cEndPoint
	//   Retorna o endpoint e porta conectada (IP ou hostname) usado pelo
	//   SmartClient. O AdvPP não mantém conexão de SmartClient, portanto
	//   devolve ""; @bBroker (por referência) não é gravável neste VM
	//   (mesma regra documentada de DBRecordInfo/DBSqlPlan).
	// =========================================================================
	natives["GETENDPOINT"] = func(args []advplrt.Value) (advplrt.Value, error) {
		return advplrt.NewString(""), nil
	}
}

// advppVersion é o placeholder devolvido por GetRmtVersion (a VM não roda um
// SmartClient real; a versão informada é a do runtime AdvPP).
const advppVersion = "17.2.0.1"

// parseBuildBlocks converte uma string de build (nnn.nnn.nnn.nnn) nos 4
// primeiros blocos numéricos; blocos ausentes ou não numéricos valem 0.
func parseBuildBlocks(s string) [4]int {
	var out [4]int
	parts := strings.Split(s, ".")
	for i := 0; i < 4 && i < len(parts); i++ {
		out[i], _ = strconv.Atoi(strings.TrimSpace(parts[i]))
	}
	return out
}

// --- Helpers de host/SO ---

func stableHostID() string {
	h := sha256.New()
	if host, err := os.Hostname(); err == nil {
		h.Write([]byte(host))
	}
	h.Write([]byte("|"))
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if data, err := os.ReadFile(p); err == nil {
			h.Write([]byte(strings.TrimSpace(string(data))))
			h.Write([]byte("|"))
		}
	}
	sum := hex.EncodeToString(h.Sum(nil))
	return strings.ToUpper(sum[:4] + "-" + sum[4:8])
}

func srvArchName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "386":
		return "i686"
	case "arm":
		return "aarch32"
	case "arm64":
		return "aarch64"
	default:
		return "unknown"
	}
}

func runtimeArchIs64() bool {
	return strings.Contains(runtime.GOARCH, "64")
}

func osName() string {
	switch runtime.GOOS {
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	case "darwin":
		return "Mac OS"
	default:
		return strings.ToUpper(runtime.GOOS)
	}
}

// osAdditional devolve informação adicional do SO: no Linux, a 1ª linha de
// /proc/version; no Windows, o "Service Pack"/build quando disponível.
func osAdditional() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/version"); err == nil {
			if line := strings.SplitN(string(data), "\n", 2); len(line) > 0 {
				return line[0]
			}
		}
	}
	if v := os.Getenv("OSVERSION"); v != "" {
		return v
	}
	return ""
}

func osVersion() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/version"); err == nil {
			if line := strings.SplitN(string(data), "\n", 2); len(line) > 0 {
				return line[0]
			}
		}
	}
	return fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
}

func osPlatform() string {
	return fmt.Sprintf("%s (%s)", runtime.GOOS, srvArchName())
}

func currentLocale() string {
	for _, k := range []string{"LANG", "LC_ALL", "LC_CTYPE"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func currentLocaleName() string {
	if v := os.Getenv("LANGUAGE"); v != "" {
		return v
	}
	return currentLocale()
}

func currentUsername() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}

func cpuMHz() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "cpu MHz") {
					fields := strings.Fields(strings.SplitN(line, ":", 2)[1])
					if len(fields) > 0 {
						return fields[0]
					}
				}
			}
		}
	}
	return ""
}

func cpuDesc() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "model name") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		}
	}
	return runtime.GOARCH
}

func localInterfaceIPs() []net.IP {
	var ips []net.IP
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		ips = append(ips, ip)
	}
	return ips
}

func allInterfaceIPs() []net.IP {
	var ips []net.IP
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips
}

func firstNonLoopbackIP() string {
	ips := localInterfaceIPs()
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	if len(ips) > 0 {
		return ips[0].String()
	}
	return ""
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		return !ip4.IsPrivate() && !ip4.IsLoopback() &&
			!ip4.IsLinkLocalMulticast() && !ip4.IsLinkLocalUnicast()
	}
	return !ip.IsPrivate() && !ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() && !ip.IsLoopback()
}

// --- Memória (Linux /proc, fallback runtime.MemStats) ---

func readMeminfo() map[string]float64 {
	m := map[string]float64{}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) == 0 {
			continue
		}
		if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
			m[strings.TrimSpace(parts[0])] = v
		}
	}
	return m
}

func memTotalMB() string {
	mi := readMeminfo()
	if total, ok := mi["MemTotal"]; ok {
		return fmt.Sprintf("%.2f MB", total/1024)
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return fmt.Sprintf("%.2f MB", float64(ms.Sys)/1024/1024)
}

func memSummary() string {
	mi := readMeminfo()
	total, hasT := mi["MemTotal"]
	free, hasF := mi["MemFree"]
	avail, hasA := mi["MemAvailable"]
	if !hasT || (!hasF && !hasA) {
		// Sem /proc/meminfo (ex.: Windows): fallback honesto com MemStats.
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		total = float64(ms.Sys) / 1024
		used := float64(ms.Alloc) / 1024
		return fmt.Sprintf("Physical memory . %10.2f MB.   Used %10.2f MB.   Free %10.2f MB.\n",
			total/1024, used/1024, (total-used)/1024)
	}
	if !hasF {
		free = avail
	}
	if hasA && avail < free {
		free = avail
	}
	used := total - free

	swapTotal, swapFree := float64(0), float64(0)
	if v, ok := mi["SwapTotal"]; ok {
		swapTotal = v
	}
	if v, ok := mi["SwapFree"]; ok {
		swapFree = v
	}
	swapUsed := swapTotal - swapFree

	return fmt.Sprintf(
		"Physical memory . %10.2f MB.   Used %10.2f MB.   Free %10.2f MB.\n"+
			"Paging file ..... %10.2f MB.   Used %10.2f MB.   Free %10.2f MB.",
		total/1024, used/1024, free/1024,
		swapTotal/1024, swapUsed/1024, swapFree/1024)
}

// procSelfStatusKB lê VmRSS/VmSize (em kB) do processo atual via
// /proc/self/status (Linux); em outros SOs usa runtime.MemStats.
func procSelfStatusKB() (residentKB, virtualKB float64) {
	if data, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			switch {
			case strings.HasPrefix(line, "VmRSS:"):
				fmt.Sscanf(line, "VmRSS: %f kB", &residentKB)
			case strings.HasPrefix(line, "VmSize:"):
				fmt.Sscanf(line, "VmSize: %f kB", &virtualKB)
			}
		}
		if residentKB > 0 || virtualKB > 0 {
			return residentKB, virtualKB
		}
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return float64(ms.Alloc) / 1024, float64(ms.Sys) / 1024
}

// --- GetSrvInfo / GetRmtInfo / GetUserInfoArray ---

func srvInfoArray() []advplrt.Value {
	return []advplrt.Value{
		advplrt.NewString(hostnameOrEmpty()),                   // 1: nome do servidor
		advplrt.NewString(osName()),                            // 2: SO
		advplrt.NewString(osAdditional()),                      // 3: info adicional do SO
		advplrt.NewString(memTotalMB()),                        // 4: memória
		advplrt.NewString(fmt.Sprintf("%d", runtime.NumCPU())), // 5: n. processadores
		advplrt.NewString(cpuMHz()),                            // 6: velocidade
		advplrt.NewString(cpuDesc()),                           // 7: identificação
		advplrt.NewString(currentLocale()),                     // 8: locale
		advplrt.NewString(currentLocaleName()),                 // 9: nome do locale
		advplrt.NewString(stableHostID()),                      // 10: SMBIOS UUID / Host ID
		advplrt.NewArray(interfaceRows()),                      // 11: interfaces [nome, MAC]
		advplrt.NewString(srvArchName()),                       // 12: arquitetura
		advplrt.NewString(srvOSDetailsJSON()),                  // 13: detalhes do SO (JSON)
	}
}

func remoteInfoArray() []advplrt.Value {
	return []advplrt.Value{
		advplrt.NewString(hostnameOrEmpty()),                   // 1: nome do computador
		advplrt.NewString(osName()),                            // 2: SO
		advplrt.NewString(osAdditional()),                      // 3: info adicional
		advplrt.NewString(memTotalMB()),                        // 4: memória física
		advplrt.NewString(fmt.Sprintf("%d", runtime.NumCPU())), // 5: processadores
		advplrt.NewString(cpuMHz()),                            // 6: MHZ
		advplrt.NewString(cpuDesc()),                           // 7: descrição
		advplrt.NewString(currentLocale()),                     // 8: linguagem
		advplrt.NewString(""),                                  // 9: navegador/marca (não web)
		advplrt.NewString(""),                                  // 10: Android/iOS (não móvel)
		advplrt.NewString(srvArchName()),                       // 11: arquitetura
		advplrt.NewString("Estatico"),                          // 12: SC estático/dinâmico
		advplrt.NewString(executableDir()),                     // 13: pasta do SC
	}
}

func interfaceRows() []advplrt.Value {
	ifs, err := net.Interfaces()
	if err != nil {
		return []advplrt.Value{}
	}
	var rows []advplrt.Value
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 && len(i.HardwareAddr) == 0 {
			continue
		}
		rows = append(rows, advplrt.NewArray([]advplrt.Value{
			advplrt.NewString(i.Name),
			advplrt.NewString(i.HardwareAddr.String()),
		}))
	}
	return rows
}

func userInfoRow() []advplrt.Value {
	active := time.Since(ambienteProcessStart)
	hh := int(active.Hours())
	mm := int(active.Minutes()) % 60
	ss := int(active.Seconds()) % 60
	resident, _ := procSelfStatusKB()
	return []advplrt.Value{
		advplrt.NewString(currentUsername()),                                  // 1: usuário
		advplrt.NewString(hostnameOrEmpty()),                                  // 2: máquina local
		advplrt.NewNumber(float64(os.Getpid())),                               // 3: ID da thread
		advplrt.NewString(""),                                                 // 4: servidor (balance)
		advplrt.NewString("ADVPP"),                                            // 5: função em execução
		advplrt.NewString(currentEnvironmentName()),                           // 6: ambiente
		advplrt.NewString(ambienteProcessStart.Format("02/01/2006 15:04:05")), // 7: conexão
		advplrt.NewString(fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)),          // 8: tempo ativo
		advplrt.NewNumber(0),                                                  // 9: instruções
		advplrt.NewNumber(0),                                                  // 10: instruções/s
		advplrt.NewString(""),                                                 // 11: observações
		advplrt.NewNumber(resident * 1024),                                    // 12: memória (bytes)
		advplrt.NewString(""),                                                 // 13: SID
		advplrt.NewNumber(0),                                                  // 14: pid ctreeserver/boundserver
		advplrt.NewString("JOB"),                                              // 15: tipo da thread
		advplrt.NewString(""),                                                 // 16: inatividade
	}
}

func currentEnvironmentName() string {
	for _, k := range []string{"PROTHEUS_ENVIRONMENT", "GETENVSERVER"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return "Default"
}

func hostnameOrEmpty() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return ""
}

func executableDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return ""
}

func srvOSDetailsJSON() string {
	details := map[string]string{
		"os":         runtime.GOOS,
		"arch":       srvArchName(),
		"version":    osVersion(),
		"platform":   osPlatform(),
		"hostname":   hostnameOrEmpty(),
		"go_version": runtime.Version(),
	}
	b, err := json.Marshal(details)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func srvOSInfo() string {
	return fmt.Sprintf(
		"OS Version .........: %s\nOS Platform ........: %s\nOS Version Info ....: %s",
		osVersion(), osPlatform(), osAdditional())
}

func srvGlbInfo() string {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	mi := readMeminfo()
	total, _ := mi["MemTotal"]
	free, _ := mi["MemFree"]
	avail, _ := mi["MemAvailable"]
	if avail > 0 && free > avail {
		free = avail
	}
	used := total - free

	var b strings.Builder
	fmt.Fprintf(&b, "----------- Total Thread Count ------------\n")
	fmt.Fprintf(&b, "                 Total Threads ... %d\n", runtime.NumGoroutine())
	fmt.Fprintf(&b, "                        Thread ... %d\n", runtime.NumGoroutine())
	fmt.Fprintf(&b, "           Global List Info -------------\n")
	fmt.Fprintf(&b, "      SymTab List ...       %.2f KB. Count %d\n", float64(ms.Sys)/1024, int(ms.HeapObjects))
	fmt.Fprintf(&b, "----------- OS Memory Summary -------------\n")
	fmt.Fprintf(&b, "Physical memory . %10.2f MB.    Used %10.2f MB.   Free %10.2f MB.\n", total/1024, used/1024, free/1024)
	fmt.Fprintf(&b, "----------- APP Memory Summary ------------\n")
	fmt.Fprintf(&b, "Service Resident Memory ... %10.2f MB.\n", float64(ms.Alloc)/1024/1024)
	return b.String()
}

// --- Impressoras / plano de energia / linhas ---

func listPrinters() []string {
	if runtime.GOOS == "windows" {
		// AdvPP não tem acesso à API Win32 de impressoras via stdlib.
		return nil
	}
	out, err := exec.Command("lpstat", "-p").Output()
	if err != nil {
		return nil
	}
	var printers []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "printer" {
			printers = append(printers, fields[1])
		}
	}
	return printers
}

func powerSchemes() []advplrt.Value {
	if runtime.GOOS != "linux" {
		return []advplrt.Value{}
	}
	n := runtime.NumCPU()
	out := make([]advplrt.Value, 0, n)
	for cpu := 0; cpu < n; cpu++ {
		name := "Unknown Scheme"
		typ := 0
		p := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/scaling_governor", cpu)
		if data, err := os.ReadFile(p); err == nil {
			switch strings.TrimSpace(string(data)) {
			case "performance":
				name, typ = "performance", 5
			case "powersave":
				name, typ = "powersave", 6
			case "userspace":
				name, typ = "userspace", 7
			case "ondemand":
				name, typ = "ondemand", 8
			case "conservative":
				name, typ = "conservative", 9
			case "schedutil":
				name, typ = "schedutil", 10
			}
		}
		out = append(out, advplrt.NewArray([]advplrt.Value{
			advplrt.NewString(name),
			advplrt.NewNumber(float64(typ)),
			advplrt.NewNumber(float64(cpu)),
		}))
	}
	return out
}

func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n := strings.Count(string(data), "\n")
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n, nil
}

// --- ShowInfMem ---

// memPools retorna linhas de [kb, count] derivadas de runtime.MemStats —
// espelho honesto (embora simplificado) dos pools do Smartheap do Protheus.
func memPools() [][2]float64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return [][2]float64{
		{float64(ms.HeapAlloc) / 1024, float64(ms.HeapObjects)},
		{float64(ms.HeapSys) / 1024, float64(ms.HeapIdle)},
		{float64(ms.StackInuse) / 1024, float64(ms.StackSys)},
		{float64(ms.GCSys) / 1024, float64(ms.NumGC)},
		{float64(ms.OtherSys) / 1024, 0},
		{float64(ms.Sys) / 1024, 0},
	}
}

func fillMemPoolArray(a *advplrt.ArrayValue) {
	pools := memPools()
	for i := 0; i < len(a.Elements); i++ {
		row := []advplrt.Value{advplrt.NewNumber(0), advplrt.NewNumber(0)}
		if i < len(pools) {
			row[0] = advplrt.NewNumber(pools[i][0])
			row[1] = advplrt.NewNumber(pools[i][1])
		}
		a.Elements[i] = advplrt.NewArray(row)
	}
}

// --- Métricas (MetricsName / MetricsRead) ---

type metricJSON struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	CollectedIn string      `json:"collected_in"`
	Unit        string      `json:"unit,omitempty"`
	Value       interface{} `json:"value,omitempty"`
	Error       string      `json:"error,omitempty"`
}

func metricNames() []string {
	return []string{
		"memory_resident", "memory_virtual", "memory_ram_total",
		"memory_ram_free", "memory_ram_used", "memory_swap_total",
		"memory_swap_used", "memory_swap_free", "startdate",
	}
}

func collectMetrics(filter []string) []metricJSON {
	resident, virtual := procSelfStatusKB()
	mi := readMeminfo()
	now := time.Now().Format("02/01/2006 15:04:05.000")

	total, _ := mi["MemTotal"]
	avail, _ := mi["MemAvailable"]
	free, _ := mi["MemFree"]
	if avail > 0 && free > avail {
		free = avail
	}
	if avail == 0 && free > 0 {
		avail = free
	}
	ramFree := avail
	if ramFree == 0 {
		ramFree = free
	}
	swapTotal, _ := mi["SwapTotal"]
	swapFree, _ := mi["SwapFree"]

	all := []metricJSON{
		{Name: "memory_resident", Description: "Resident Memory Usage", CollectedIn: now, Unit: "kb", Value: intRound(resident)},
		{Name: "memory_virtual", Description: "Virtual Memory Usage", CollectedIn: now, Unit: "kb", Value: intRound(virtual)},
		{Name: "memory_ram_total", Description: "Memory Ram Total", CollectedIn: now, Unit: "kb", Value: intRound(total)},
		{Name: "memory_ram_free", Description: "Memory Ram Free (Available to Use)", CollectedIn: now, Unit: "kb", Value: intRound(ramFree)},
		{Name: "memory_ram_used", Description: "Memory Ram Usage", CollectedIn: now, Unit: "kb", Value: intRound(total - ramFree)},
		{Name: "memory_swap_total", Description: "Page File Total", CollectedIn: now, Unit: "kb", Value: intRound(swapTotal)},
		{Name: "memory_swap_used", Description: "Page File Used", CollectedIn: now, Unit: "kb", Value: intRound(swapTotal - swapFree)},
		{Name: "memory_swap_free", Description: "Page File Free (Available to Use)", CollectedIn: now, Unit: "kb", Value: intRound(swapFree)},
		{Name: "startdate", Description: "Date when the system was started", CollectedIn: now, Value: ambienteProcessStart.Format("02/01/2006 15:04:05")},
	}

	if len(filter) == 0 {
		return all
	}
	valid := make(map[string]bool, len(all))
	for _, m := range all {
		valid[m.Name] = true
	}
	out := make([]metricJSON, 0, len(filter))
	for _, f := range filter {
		if !valid[f] {
			out = append(out, metricJSON{Name: f, Error: "invalid metric", CollectedIn: now})
			continue
		}
		for _, m := range all {
			if m.Name == f {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

func intRound(f float64) int64 {
	return int64(math.Round(f))
}

// --- UUID ---

func newUUIDv4() (string, bool) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", false
	}
	b[6] = (b[6] & 0x0F) | 0x40 // versão 4
	b[8] = (b[8] & 0x3F) | 0x80 // variante 10
	return formatUUID(b), true
}

func newUUIDSeq() (string, bool) {
	seq := atomic.AddUint64(&ambienteUUIDSeqCounter, 1)
	var b [16]byte
	if _, err := rand.Read(b[8:]); err != nil {
		return "", false
	}
	// Contador monotônico big-endian nos 6 bytes iniciais (48 bits), que nunca
	// são mascarados: UUIDs gerados em sequência crescem lexicograficamente
	// (semântica "sequencial"). Os bytes 6-7 (time_hi/ver) e 8 (variante) são
	// fixos, os demais aleatórios.
	b[0] = byte(seq >> 40)
	b[1] = byte(seq >> 32)
	b[2] = byte(seq >> 24)
	b[3] = byte(seq >> 16)
	b[4] = byte(seq >> 8)
	b[5] = byte(seq)
	b[6] = 0x40 // versão 4
	b[7] = 0x00
	b[8] = (b[8] & 0x3F) | 0x80 // variante 10
	return formatUUID(b), true
}

func formatUUID(b [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
