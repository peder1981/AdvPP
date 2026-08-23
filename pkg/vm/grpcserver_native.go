package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// GRPCServer: gRPC embarcado no AdvPP (framework real, google.golang.org/grpc
// — a mesma lib usada por tGrpc no lado cliente), simétrico ao WSRestServer
// (pkg/vm/rest_native.go) que já expõe User Functions como rotas HTTP REST.
//
// AdvPL/TLPP não tem como declarar tipos de mensagem .proto estaticamente
// (não existe um DSL de schema protobuf na linguagem) — por isso, assim
// como WSRestServer serializa parâmetros/retorno como JSON sobre HTTP, o
// GRPCServer define, em tempo de execução, UM ÚNICO tipo de mensagem
// genérico reaproveitado por toda RPC registrada:
//
//	message JsonEnvelope { string json = 1; }
//
// O campo "json" carrega os parâmetros da chamada (objeto JSON, mesma
// convenção de WSRestServer/MCPServer) na requisição, e o retorno da
// função no mesmo formato na resposta. Isso é HTTP/2 e protobuf reais
// (wire format real, handshake real) — só a "granularidade" do schema é
// genérica, não simulada: qualquer cliente gRPC real (grpcurl, tGrpc deste
// mesmo compilador via Server Reflection, um cliente gerado por protoc)
// consegue descobrir e chamar o serviço, desde que saiba (ou descubra via
// reflection, que este servidor também expõe) que o campo se chama "json".
type grpcMethodReg struct {
	service  string
	method   string
	funcName string
}

type grpcServerState struct {
	registrations []grpcMethodReg
	server        *grpc.Server
	listener      net.Listener
}

var grpcServerInstanceSeq int64

func newGRPCServerObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("GRPCSERVER", nil)
	obj.Native = &grpcServerState{}
	return obj
}

// grpcServerBuildFileDescriptor monta, a partir das rotas registradas via
// AddMethod, um FileDescriptorProto real (package exclusivo desta
// instância — evita colisão de nomes com outro GRPCServer no mesmo
// processo, ex.: em testes que sobem vários servidores) com:
//   - a mensagem JsonEnvelope compartilhada por todas as RPCs
//   - um ServiceDescriptorProto por nome de serviço distinto registrado,
//     cada um com um MethodDescriptorProto por método registrado nele
//
// Registra o arquivo em protoregistry.GlobalFiles (necessário para que
// reflection.Register — usado por qualquer cliente, inclusive tGrpc deste
// compilador — encontre os descritores) e devolve o FileDescriptor real
// resultante.
func grpcServerBuildFileDescriptor(pkg string, regs []grpcMethodReg) (protoreflect.FileDescriptor, error) {
	strp := func(s string) *string { return &s }
	i32p := func(i int32) *int32 { return &i }
	lbl := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	strType := descriptorpb.FieldDescriptorProto_TYPE_STRING

	envelopeMsg := &descriptorpb.DescriptorProto{
		Name: strp("JsonEnvelope"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{Name: strp("json"), Number: i32p(1), Type: strType.Enum(), Label: &lbl, JsonName: strp("json")},
		},
	}

	byService := map[string][]*descriptorpb.MethodDescriptorProto{}
	var order []string
	for _, r := range regs {
		if _, seen := byService[r.service]; !seen {
			order = append(order, r.service)
		}
		byService[r.service] = append(byService[r.service], &descriptorpb.MethodDescriptorProto{
			Name:       strp(r.method),
			InputType:  strp("." + pkg + ".JsonEnvelope"),
			OutputType: strp("." + pkg + ".JsonEnvelope"),
		})
	}

	fd := &descriptorpb.FileDescriptorProto{
		Name:        strp(pkg + ".proto"),
		Package:     strp(pkg),
		Syntax:      strp("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{envelopeMsg},
	}
	for _, svcName := range order {
		fd.Service = append(fd.Service, &descriptorpb.ServiceDescriptorProto{
			Name:   strp(svcName),
			Method: byService[svcName],
		})
	}

	f, err := protodesc.NewFile(fd, protoregistry.GlobalFiles)
	if err != nil {
		return nil, fmt.Errorf("montagem do descritor .proto: %w", err)
	}
	if err := protoregistry.GlobalFiles.RegisterFile(f); err != nil {
		return nil, fmt.Errorf("registro do descritor .proto: %w", err)
	}
	return f, nil
}

// grpcServerHandlerFor cria o grpc.MethodHandler que decodifica o campo
// "json" da requisição, despacha para funcName numa VM isolada (mesmo
// motivo de restHandlerFor: v.RunFunction direto não é seguro aqui —
// reentraria na pilha de chamadas da VM bloqueada dentro de Serve()) e
// devolve o retorno serializado de volta no campo "json" da resposta.
func (v *VM) grpcServerHandlerFor(envelopeDesc protoreflect.MessageDescriptor, funcName string) grpc.MethodHandler {
	jsonField := envelopeDesc.Fields().ByName("json")
	return func(_ interface{}, _ context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
		req := dynamicpb.NewMessage(envelopeDesc)
		if err := dec(req); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "decode: %v", err)
		}
		jsonStr := req.Get(jsonField).String()

		// jsonToAdvplValue (não jsonMapToAdvplObject) preserva o case
		// original das chaves — consistente com o acesso por colchete
		// `oParams["campo"]` (case-sensitive, "semântica JSON", ver
		// OP_ARRAY_GET em vm.go) que é o jeito natural de ler um payload
		// externo arbitrário; jsonMapToAdvplObject maiusculiza para
		// suportar `oArgs:CAMPO` (uso documentado do MCPServer, onde os
		// nomes de argumento vêm de um schema conhecido) — não é o
		// convênio certo aqui, onde o payload é JSON externo genérico.
		var params any
		if jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &params); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "campo json inválido: %v", err)
			}
		}
		argObj := jsonToAdvplValue(params)

		job := NewVM(v.bc, false)
		job.dbFactory = v.dbFactory
		if v.dbFactory != nil {
			job.dbEngine = v.dbFactory()
		}
		result, err := job.RunFunction(funcName, []advplrt.Value{argObj})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}

		respBytes, err := json.Marshal(advplValueToJSON(result))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encode do retorno: %v", err)
		}
		resp := dynamicpb.NewMessage(envelopeDesc)
		resp.Set(jsonField, protoreflect.ValueOfString(string(respBytes)))
		return resp, nil
	}
}

func (v *VM) callGRPCServerMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*grpcServerState)
	if !ok {
		return fmt.Errorf("GRPCServer: objeto sem estado interno")
	}

	switch method {
	case "NEW":
		v.push(obj)
		return nil

	case "ADDMETHOD":
		// AddMethod( < cServiceName >, < cMethodName >, < cFuncName > ) -> Nil
		// Registra funcName (User Function existente no bytecode) como
		// handler unário do método cMethodName do serviço cServiceName.
		// Deve ser chamado antes de Serve() — grpc.Server não permite
		// registrar serviços depois de iniciado.
		if st.server != nil {
			return fmt.Errorf("GRPCServer:AddMethod: não é possível registrar métodos depois de Serve()")
		}
		svc := advplrt.ToString(getArg(args, 0))
		mth := advplrt.ToString(getArg(args, 1))
		fn := advplrt.ToString(getArg(args, 2))
		if svc == "" || mth == "" || fn == "" {
			return fmt.Errorf("GRPCServer:AddMethod: cServiceName/cMethodName/cFuncName obrigatórios")
		}
		st.registrations = append(st.registrations, grpcMethodReg{service: svc, method: mth, funcName: fn})
		v.push(advplrt.Nil)
		return nil

	case "SERVE":
		// Serve( [ nPort ] ) -> Nil (bloqueante, mesmo padrão de
		// WSRestServer:Serve — retorna quando Shutdown() é chamado de
		// outra goroutine/job, ou em erro real de bind/listen).
		if len(st.registrations) == 0 {
			return fmt.Errorf("GRPCServer:Serve: nenhum método registrado (chame AddMethod antes)")
		}
		addr := ":50051"
		if len(args) > 0 && args[0] != nil && args[0] != advplrt.Nil {
			switch a := args[0].(type) {
			case *advplrt.NumberValue:
				addr = fmt.Sprintf(":%d", int(a.Val))
			case *advplrt.StringValue:
				if _, err := strconv.Atoi(a.Val); err == nil {
					addr = ":" + a.Val
				} else {
					addr = a.Val
				}
			}
		}

		pkg := fmt.Sprintf("advpp.srv%d", atomic.AddInt64(&grpcServerInstanceSeq, 1))
		file, err := grpcServerBuildFileDescriptor(pkg, st.registrations)
		if err != nil {
			return fmt.Errorf("GRPCServer:Serve: %w", err)
		}
		envelopeDesc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(pkg + ".JsonEnvelope"))
		if err != nil {
			return fmt.Errorf("GRPCServer:Serve: %w", err)
		}
		msgDesc := envelopeDesc.(protoreflect.MessageDescriptor)

		s := grpc.NewServer()
		svcs := file.Services()
		for i := 0; i < svcs.Len(); i++ {
			svc := svcs.Get(i)
			desc := &grpc.ServiceDesc{
				ServiceName: string(svc.FullName()),
				HandlerType: (*any)(nil),
				Metadata:    pkg + ".proto",
			}
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				funcName := grpcServerFindFunc(st.registrations, string(svc.Name()), string(m.Name()))
				desc.Methods = append(desc.Methods, grpc.MethodDesc{
					MethodName: string(m.Name()),
					Handler:    v.grpcServerHandlerFor(msgDesc, funcName),
				})
			}
			s.RegisterService(desc, struct{}{})
		}
		reflection.Register(s)

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("GRPCServer:Serve: %w", err)
		}
		st.server = s
		st.listener = ln

		if err := s.Serve(ln); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			return fmt.Errorf("GRPCServer:Serve: %w", err)
		}
		v.push(advplrt.Nil)
		return nil

	case "SHUTDOWN":
		if st.server != nil {
			st.server.GracefulStop()
		}
		v.push(advplrt.Nil)
		return nil

	default:
		return fmt.Errorf("GRPCServer: método desconhecido %q", method)
	}
}

// grpcServerFindFunc localiza a função registrada para (serviceName,
// methodName) — sempre encontra (a lista de métodos do FileDescriptor
// vem exatamente das mesmas registrations).
func grpcServerFindFunc(regs []grpcMethodReg, service, methodName string) string {
	for _, r := range regs {
		if r.service == service && r.method == methodName {
			return r.funcName
		}
	}
	return ""
}
