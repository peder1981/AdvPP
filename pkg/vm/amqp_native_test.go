package vm

import (
	"net"
	"testing"
	"time"

	"github.com/advpl/compiler/pkg/compiler"
	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// skipIfNoBroker evita quebrar CI/ambientes sem RabbitMQ local; este teste é
// de integração real (não mock), exige um broker em localhost:5672.
func skipIfNoBroker(t *testing.T) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", "localhost:5672", 500*time.Millisecond)
	if err != nil {
		t.Skip("sem broker AMQP em localhost:5672 (suba com: docker run -d --rm -p 5672:5672 rabbitmq:3-alpine)")
	}
	conn.Close()
}

func TestTAMQPPublishConsumeAck(t *testing.T) {
	skipIfNoBroker(t)
	v := NewVM(&compiler.Bytecode{}, false)

	sender := newAMQPObject()
	if err := v.callTAMQPMethod(sender, "NEW", []advplrt.Value{
		advplrt.NewString("localhost"), advplrt.NewNumber(5672),
		advplrt.NewString("guest"), advplrt.NewString("guest"), advplrt.NewNumber(1),
	}); err != nil {
		t.Fatalf("New: %v", err)
	}
	senderObj := v.pop().(*advplrt.ObjectValue)
	senderSt := senderObj.Native.(*amqpState)
	if senderSt.lastErr != "" {
		t.Fatalf("New falhou: %s", senderSt.lastErr)
	}

	queue := "advpp_test_queue"
	ret := mustCallAMQP(t, v, senderObj, "QUEUEDECLARE", []advplrt.Value{
		advplrt.NewString(queue), advplrt.NewBool(false), advplrt.NewBool(false), advplrt.NewBool(true), advplrt.NewBool(false),
	})
	if !advplrt.ToBool(ret) {
		t.Fatalf("QueueDeclare falhou: %s", senderSt.lastErr)
	}

	ret = mustCallAMQP(t, v, senderObj, "BASICPUBLISH", []advplrt.Value{
		advplrt.NewString(""), advplrt.NewString(queue), advplrt.NewBool(true), advplrt.NewString("hello world!"),
		advplrt.NewString("corr-1"), advplrt.NewString("reply-q"),
	})
	if !advplrt.ToBool(ret) {
		t.Fatalf("BasicPublish falhou: %s", senderSt.lastErr)
	}

	receiver := newAMQPObject()
	_ = v.callTAMQPMethod(receiver, "NEW", []advplrt.Value{
		advplrt.NewString("localhost"), advplrt.NewNumber(5672),
		advplrt.NewString("guest"), advplrt.NewString("guest"), advplrt.NewNumber(1),
	})
	receiverObj := v.pop().(*advplrt.ObjectValue)

	ret = mustCallAMQP(t, v, receiverObj, "BASICCONSUME", []advplrt.Value{
		advplrt.NewString(queue), advplrt.NewBool(false), advplrt.NewBool(false),
	})
	if !advplrt.ToBool(ret) {
		st := receiverObj.Native.(*amqpState)
		t.Fatalf("BasicConsume falhou: %s", st.lastErr)
	}
	body := advplrt.ToString(receiverObj.Props["BODY"])
	if body != "hello world!" {
		t.Errorf("Body = %q, quer %q", body, "hello world!")
	}
	if advplrt.ToString(mustCallAMQP(t, v, receiverObj, "CORRELATIONID", nil)) != "corr-1" {
		t.Error("CorrelationID não confere")
	}
	if advplrt.ToString(mustCallAMQP(t, v, receiverObj, "REPLYTO", nil)) != "reply-q" {
		t.Error("ReplyTo não confere")
	}

	tag := advplrt.ToString(mustCallAMQP(t, v, receiverObj, "TAG", nil))
	if tag == "" {
		t.Fatal("Tag vazia após BasicConsume")
	}
	ackRet := mustCallAMQP(t, v, receiverObj, "BASICACK", []advplrt.Value{advplrt.NewString(tag), advplrt.NewBool(false)})
	if !advplrt.ToBool(ackRet) {
		st := receiverObj.Native.(*amqpState)
		t.Fatalf("BasicAck falhou: %s", st.lastErr)
	}
}

func TestTAMQPConsumeTimeout(t *testing.T) {
	skipIfNoBroker(t)
	v := NewVM(&compiler.Bytecode{}, false)

	obj := newAMQPObject()
	_ = v.callTAMQPMethod(obj, "NEW", []advplrt.Value{
		advplrt.NewString("localhost"), advplrt.NewNumber(5672),
		advplrt.NewString("guest"), advplrt.NewString("guest"), advplrt.NewNumber(1),
	})
	obj2 := v.pop().(*advplrt.ObjectValue)

	queue := "advpp_test_empty_queue"
	_ = mustCallAMQP(t, v, obj2, "QUEUEDECLARE", []advplrt.Value{
		advplrt.NewString(queue), advplrt.NewBool(false), advplrt.NewBool(false), advplrt.NewBool(true), advplrt.NewBool(false),
	})
	obj2.SetProp("CONSUMETIMEOUT", advplrt.NewNumber(1))

	start := time.Now()
	ret := mustCallAMQP(t, v, obj2, "BASICCONSUME", []advplrt.Value{
		advplrt.NewString(queue), advplrt.NewBool(true), advplrt.NewBool(false),
	})
	elapsed := time.Since(start)
	if advplrt.ToBool(ret) {
		t.Error("BasicConsume em fila vazia deveria retornar .F. (timeout)")
	}
	if elapsed < 900*time.Millisecond || elapsed > 3*time.Second {
		t.Errorf("timeout de ConsumeTimeout=1 levou %v, esperado ~1s", elapsed)
	}
}

func mustCallAMQP(t *testing.T, v *VM, obj *advplrt.ObjectValue, method string, args []advplrt.Value) advplrt.Value {
	t.Helper()
	if err := v.callTAMQPMethod(obj, method, args); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return v.pop()
}
