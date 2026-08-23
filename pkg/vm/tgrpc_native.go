package vm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// tGrpcState é o estado Go da classe tGrpc (TDN: Não Visual / tGrpc).
//
// A TDN documenta que tGrpc fala um protocolo de mensagens proprietário
// da TOTVS ("modelo Smartlink") sobre gRPC, mas não publica o .proto
// correspondente — sem ele não há como saber de antemão nomes reais de
// serviço/método/campo. Em vez de simular sucesso ou ficar travado num
// stub sempre-falso, esta implementação usa a biblioteca gRPC real
// (google.golang.org/grpc) para: (1) abrir uma conexão HTTP/2 real e
// verificável (isRunning reflete o estado real da conexão, não um valor
// fixo); (2) descobrir em tempo de execução, via gRPC Server Reflection
// (RFC padrão do próprio gRPC, github.com/grpc/grpc/blob/master/doc/server-reflection.md),
// quais serviços/métodos o servidor alvo realmente expõe; (3) invocar
// dinamicamente (via google.golang.org/protobuf/types/dynamicpb, sem
// stubs gerados em tempo de compilação) o método cujo nome mais se
// aproxima do método documentado pela TDN (ex.: "ClientSetup" ->
// qualquer RPC chamado ClientSetup/client_setup no servidor), populando
// os campos da mensagem de entrada a partir das propriedades TDN
// (ClientInfoProp, MsgId, MsgType, ...) por casamento de nome de campo.
//
// Isso é uma invocação gRPC real (payload wire real, HTTP/2 real,
// protobuf real) contra QUALQUER servidor gRPC compatível com Server
// Reflection — não uma simulação. A limitação genuína que permanece
// (documentada, não escondida): sem o .proto do Smartlink não há garantia
// de que os nomes de método/campo do servidor real batam com os
// candidatos usados aqui; contra um servidor Smartlink real isso precisa
// ser validado e ajustado. Contra qualquer servidor gRPC de teste com
// reflection habilitado e nomes de método/campo convencionais, funciona
// de ponta a ponta (ver pkg/vm/tgrpc_native_test.go).
type tGrpcState struct {
	protoFile string
	host      string
	port      int
	conn      *grpc.ClientConn

	services []protoreflect.ServiceDescriptor // cache da descoberta via reflection

	clientInfoProp    string
	msgID             string
	msgType           string
	msgContent        string
	msgAud            string
	msgDeliveryTag    float64
	msgAckDeliveryTag float64
	msgAck            bool

	errorCode int
	errorDesc string
}

// tGrpcDialTimeout / tGrpcCallTimeout: a TDN não documenta um parâmetro de
// timeout para esta classe (diferente de outras como SFTP/DynCall) — usa-se
// um teto interno razoável para não travar o VM indefinidamente numa rede
// inacessível.
const (
	tGrpcDialTimeout = 5 * time.Second
	tGrpcCallTimeout = 10 * time.Second
)

func newTGrpcState() *tGrpcState {
	return &tGrpcState{errorCode: 0, errorDesc: ""}
}

func newTGrpcObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TGRPC", nil)
	obj.Native = newTGrpcState()
	obj.Props["CLIENTINFOPROP"] = advplrt.NewString("")
	obj.Props["MSGID"] = advplrt.NewString("")
	obj.Props["MSGTYPE"] = advplrt.NewString("")
	obj.Props["MSGCONTENT"] = advplrt.NewString("")
	obj.Props["MSGAUD"] = advplrt.NewString("")
	obj.Props["MSGDELIVERYTAG"] = advplrt.NewNumber(0)
	obj.Props["MSGACKDELIVERYTAG"] = advplrt.NewNumber(0)
	obj.Props["MSGACK"] = advplrt.False
	return obj
}

// tGrpcSyncFromProps copia as propriedades caractere/numéricas legíveis
// via `obj:Msg*`/`obj:ClientInfoProp` para o estado Go antes de um método
// que as consome (mesma convenção usada pelo exemplo da TDN: o código
// AdvPL grava a propriedade e só depois chama o método).
func tGrpcSyncFromProps(obj *advplrt.ObjectValue, st *tGrpcState) {
	st.clientInfoProp = advplrt.ToString(obj.Props["CLIENTINFOPROP"])
	st.msgID = advplrt.ToString(obj.Props["MSGID"])
	st.msgType = advplrt.ToString(obj.Props["MSGTYPE"])
	st.msgContent = advplrt.ToString(obj.Props["MSGCONTENT"])
	st.msgAud = advplrt.ToString(obj.Props["MSGAUD"])
	st.msgDeliveryTag = advplrt.ToFloat(obj.Props["MSGDELIVERYTAG"])
	st.msgAckDeliveryTag = advplrt.ToFloat(obj.Props["MSGACKDELIVERYTAG"])
	if b, ok := obj.Props["MSGACK"].(*advplrt.BoolValue); ok {
		st.msgAck = b.Val
	}
}

// connect abre (uma vez) a conexão gRPC real para host:port. grpc.NewClient
// não bloqueia — a conexão HTTP/2 real só é estabelecida sob demanda
// (Connect()/primeira chamada), verificada de verdade por isRunning/
// checkRunning abaixo via connectivity.State real.
func (st *tGrpcState) connect() error {
	if st.conn != nil {
		return nil
	}
	target := fmt.Sprintf("%s:%d", st.host, st.port)
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	st.conn = conn
	return nil
}

// checkRunning dispara a conexão real e espera (até tGrpcDialTimeout) o
// estado ficar Ready — reflete conectividade HTTP/2 real, não um valor
// fixo.
func (st *tGrpcState) checkRunning() bool {
	if err := st.connect(); err != nil {
		st.errorCode = 100
		st.errorDesc = err.Error()
		return false
	}
	st.conn.Connect()
	ctx, cancel := context.WithTimeout(context.Background(), tGrpcDialTimeout)
	defer cancel()
	for {
		state := st.conn.GetState()
		if state == connectivity.Ready {
			st.errorCode = 0
			st.errorDesc = ""
			return true
		}
		if state == connectivity.Shutdown {
			st.errorCode = 100
			st.errorDesc = "connection shutdown"
			return false
		}
		if !st.conn.WaitForStateChange(ctx, state) {
			st.errorCode = 100
			st.errorDesc = fmt.Sprintf("timeout waiting for connection (last state: %s)", state)
			return false
		}
	}
}

// discoverServices consulta o serviço padrão de gRPC Server Reflection do
// servidor alvo e monta os ServiceDescriptor reais (com FileDescriptor
// completo, incluindo dependências transitivas) via
// google.golang.org/protobuf/reflect/protodesc — permite montar/ler
// mensagens de qualquer serviço exposto sem stub gerado em tempo de
// compilação. Cacheado por instância (uma descoberta por conexão).
func (st *tGrpcState) discoverServices(ctx context.Context) ([]protoreflect.ServiceDescriptor, error) {
	if st.services != nil {
		return st.services, nil
	}
	if err := st.connect(); err != nil {
		return nil, err
	}

	client := grpc_reflection_v1.NewServerReflectionClient(st.conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.CloseSend()

	if err := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{ListServices: "*"},
	}); err != nil {
		return nil, err
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	lst := resp.GetListServicesResponse()
	if lst == nil {
		if e := resp.GetErrorResponse(); e != nil {
			return nil, fmt.Errorf("reflection ListServices: %s", e.GetErrorMessage())
		}
		return nil, fmt.Errorf("reflection: resposta inesperada a ListServices")
	}

	fdSet := &descriptorpb.FileDescriptorSet{}
	seen := map[string]bool{}
	var svcNames []string
	for _, s := range lst.GetService() {
		name := s.GetName()
		if strings.HasPrefix(name, "grpc.reflection.") {
			continue // não é um serviço de negócio do servidor alvo
		}
		svcNames = append(svcNames, name)

		if err := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: name},
		}); err != nil {
			return nil, err
		}
		fresp, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		fdr := fresp.GetFileDescriptorResponse()
		if fdr == nil {
			continue
		}
		for _, raw := range fdr.GetFileDescriptorProto() {
			var fdProto descriptorpb.FileDescriptorProto
			if err := proto.Unmarshal(raw, &fdProto); err != nil {
				continue
			}
			if seen[fdProto.GetName()] {
				continue
			}
			seen[fdProto.GetName()] = true
			fdSet.File = append(fdSet.File, &fdProto)
		}
	}
	if len(svcNames) == 0 {
		return nil, fmt.Errorf("reflection: nenhum serviço de negócio exposto pelo servidor")
	}

	files, err := protodesc.NewFiles(fdSet)
	if err != nil {
		return nil, fmt.Errorf("reflection: falha ao montar descritores: %w", err)
	}

	var services []protoreflect.ServiceDescriptor
	for _, name := range svcNames {
		d, err := files.FindDescriptorByName(protoreflect.FullName(name))
		if err != nil {
			continue
		}
		if sd, ok := d.(protoreflect.ServiceDescriptor); ok {
			services = append(services, sd)
		}
	}
	st.services = services
	return services, nil
}

// grpcMethodNameMatches compara ignorando maiúsculas/minúsculas e "_"
// (dois estilos comuns de nomeação .proto: PascalCase e snake_case).
func grpcMethodNameMatches(name string, candidates ...string) bool {
	norm := strings.ToLower(strings.ReplaceAll(name, "_", ""))
	for _, c := range candidates {
		if norm == strings.ToLower(strings.ReplaceAll(c, "_", "")) {
			return true
		}
	}
	return false
}

// findMethod procura, entre todos os serviços descobertos via reflection,
// o primeiro método cujo nome bate com algum dos candidatos (aliases do
// nome documentado pela TDN).
func (st *tGrpcState) findMethod(ctx context.Context, candidates ...string) (protoreflect.MethodDescriptor, error) {
	services, err := st.discoverServices(ctx)
	if err != nil {
		return nil, err
	}
	for _, svc := range services {
		methods := svc.Methods()
		for i := 0; i < methods.Len(); i++ {
			m := methods.Get(i)
			if grpcMethodNameMatches(string(m.Name()), candidates...) {
				return m, nil
			}
		}
	}
	return nil, fmt.Errorf("nenhum método no servidor bate com %v (reflection encontrou %d serviço(s))", candidates, len(services))
}

// grpcFieldNormAliases mapeia nomes de campo .proto normalizados
// (minúsculo, sem "_") para os dados de estado da TDN — usado tanto para
// popular a mensagem de request quanto para ler a de response.
var grpcFieldAliases = map[string]string{
	"clientinfoprop": "clientinfoprop", "clientinfo": "clientinfoprop", "info": "clientinfoprop", "prop": "clientinfoprop",
	"tenantid": "clientinfoprop", "tenant": "clientinfoprop",
	"msgid": "msgid", "id": "msgid",
	"msgtype": "msgtype", "type": "msgtype",
	"msgcontent": "msgcontent", "content": "msgcontent", "message": "msgcontent", "body": "msgcontent",
	"msgaud": "msgaud", "aud": "msgaud", "audience": "msgaud",
	"msgdeliverytag": "msgdeliverytag", "deliverytag": "msgdeliverytag",
	"msgackdeliverytag": "msgackdeliverytag", "ackdeliverytag": "msgackdeliverytag",
	"msgack": "msgack", "ack": "msgack",
}

// grpcPopulateRequest preenche os campos da mensagem de request cujo nome
// (normalizado) corresponde a uma propriedade TDN conhecida do estado
// atual do objeto.
func grpcPopulateRequest(msg *dynamicpb.Message, st *tGrpcState) {
	fields := msg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		norm := strings.ToLower(strings.ReplaceAll(string(fd.Name()), "_", ""))
		target, ok := grpcFieldAliases[norm]
		if !ok {
			continue
		}
		switch target {
		case "clientinfoprop":
			grpcSetStringField(msg, fd, st.clientInfoProp)
		case "msgid":
			grpcSetStringField(msg, fd, st.msgID)
		case "msgtype":
			grpcSetStringField(msg, fd, st.msgType)
		case "msgcontent":
			grpcSetStringField(msg, fd, st.msgContent)
		case "msgaud":
			grpcSetStringField(msg, fd, st.msgAud)
		case "msgdeliverytag":
			grpcSetNumField(msg, fd, st.msgDeliveryTag)
		case "msgackdeliverytag":
			grpcSetNumField(msg, fd, st.msgAckDeliveryTag)
		case "msgack":
			if fd.Kind() == protoreflect.BoolKind {
				msg.Set(fd, protoreflect.ValueOfBool(st.msgAck))
			}
		}
	}
}

func grpcSetStringField(msg *dynamicpb.Message, fd protoreflect.FieldDescriptor, val string) {
	if fd.Kind() == protoreflect.StringKind && !fd.IsList() {
		msg.Set(fd, protoreflect.ValueOfString(val))
	}
}

func grpcSetNumField(msg *dynamicpb.Message, fd protoreflect.FieldDescriptor, val float64) {
	if fd.IsList() {
		return
	}
	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		msg.Set(fd, protoreflect.ValueOfInt32(int32(val)))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		msg.Set(fd, protoreflect.ValueOfInt64(int64(val)))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		msg.Set(fd, protoreflect.ValueOfUint32(uint32(val)))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		msg.Set(fd, protoreflect.ValueOfUint64(uint64(val)))
	case protoreflect.FloatKind:
		msg.Set(fd, protoreflect.ValueOfFloat32(float32(val)))
	case protoreflect.DoubleKind:
		msg.Set(fd, protoreflect.ValueOfFloat64(val))
	}
}

// grpcExtractContent lê da mensagem de response o primeiro campo string
// cujo nome normalizado bate com um alias de "conteúdo de mensagem"
// (usado por waitForMessages).
func grpcExtractContent(msg *dynamicpb.Message) string {
	fields := msg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		norm := strings.ToLower(strings.ReplaceAll(string(fd.Name()), "_", ""))
		if grpcFieldAliases[norm] == "msgcontent" && fd.Kind() == protoreflect.StringKind {
			return msg.Get(fd).String()
		}
	}
	return ""
}

// invokeMethod monta a mensagem de request (populada via
// grpcPopulateRequest), invoca a RPC unária real via conn.Invoke (payload
// protobuf real, sem stub gerado em tempo de compilação) e devolve a
// mensagem de response dinâmica.
func (st *tGrpcState) invokeMethod(ctx context.Context, md protoreflect.MethodDescriptor) (*dynamicpb.Message, error) {
	req := dynamicpb.NewMessage(md.Input())
	grpcPopulateRequest(req, st)
	resp := dynamicpb.NewMessage(md.Output())

	fullMethod := fmt.Sprintf("/%s/%s", md.Parent().FullName(), md.Name())
	if err := st.conn.Invoke(ctx, fullMethod, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// callByName encapsula o fluxo comum de find+invoke usado por
// clientSetup/tenantSetup/tenantUndo/sendMessage/ackMessage: procura um
// método cujo nome bate com `candidates`, invoca com os campos do estado
// atual e devolve .T. em sucesso (registrando erro em ErrorCode/ErrorDesc
// em caso de falha).
func (v *VM) grpcCallByName(obj *advplrt.ObjectValue, st *tGrpcState, candidates ...string) (*dynamicpb.Message, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), tGrpcCallTimeout)
	defer cancel()

	md, err := st.findMethod(ctx, candidates...)
	if err != nil {
		st.errorCode = 101
		st.errorDesc = err.Error()
		return nil, false
	}
	resp, err := st.invokeMethod(ctx, md)
	if err != nil {
		st.errorCode = 102
		st.errorDesc = err.Error()
		return nil, false
	}
	st.errorCode = 0
	st.errorDesc = ""
	return resp, true
}

func (v *VM) callTGrpcMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*tGrpcState)
	if !ok {
		return fmt.Errorf("tGrpc: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		// New( < cProtoFile >, < cHost >, < nPort > ) -> objeto
		// cProtoFile é armazenado (identifica o schema pretendido) mas não
		// é compilado/lido — a descoberta de serviços/métodos reais usa
		// gRPC Server Reflection em tempo de execução (ver discoverServices).
		st.protoFile = advplrt.ToString(getArg(args, 0))
		st.host = advplrt.ToString(getArg(args, 1))
		st.port = int(advplrt.ToFloat(getArg(args, 2)))
		v.push(obj)
		return nil

	case "ISRUNNING":
		v.push(advplrt.NewBool(st.checkRunning()))
		return nil

	case "CLIENTSETUP":
		tGrpcSyncFromProps(obj, st)
		_, ok := v.grpcCallByName(obj, st, "ClientSetup")
		v.push(advplrt.NewBool(ok))
		return nil

	case "TENANTSETUP":
		tGrpcSyncFromProps(obj, st)
		_, ok := v.grpcCallByName(obj, st, "TenantSetup")
		v.push(advplrt.NewBool(ok))
		return nil

	case "TENANTUNDO":
		tGrpcSyncFromProps(obj, st)
		_, ok := v.grpcCallByName(obj, st, "TenantUndo")
		v.push(advplrt.NewBool(ok))
		return nil

	case "SENDMESSAGE", "SENDMESSAGES":
		tGrpcSyncFromProps(obj, st)
		_, ok := v.grpcCallByName(obj, st, "SendMessage", "SendMessages")
		v.push(advplrt.NewBool(ok))
		return nil

	case "WAITFORMESSAGES":
		tGrpcSyncFromProps(obj, st)
		resp, ok := v.grpcCallByName(obj, st, "WaitForMessages", "WaitForMessage", "Receive", "ReceiveMessage")
		if !ok {
			v.push(advplrt.NewString(""))
			return nil
		}
		content := grpcExtractContent(resp)
		if content != "" {
			st.msgContent = content
			obj.Props["MSGCONTENT"] = advplrt.NewString(content)
		}
		v.push(advplrt.NewString(content))
		return nil

	case "ACKMESSAGE":
		tGrpcSyncFromProps(obj, st)
		_, ok := v.grpcCallByName(obj, st, "AckMessage", "Ack")
		v.push(advplrt.NewBool(ok))
		return nil

	case "ERRORCODE":
		v.push(advplrt.NewNumber(float64(st.errorCode)))
		return nil

	case "ERRORDESC":
		v.push(advplrt.NewString(st.errorDesc))
		return nil

	default:
		return fmt.Errorf("unknown method %s on tGrpc", method)
	}
}
