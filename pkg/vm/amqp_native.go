package vm

import (
	"context"
	"fmt"
	"net/url"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	advplrt "github.com/advpl/compiler/pkg/runtime"
)

// amqpState é o estado Go da classe tAMQP (TDN: Classes/Componentes/Nao-Visual/tAMQP,
// wrapper AMQP 0-9-1 sobre github.com/rabbitmq/amqp091-go — dependência nova,
// aprovada explicitamente pelo usuário para este native, por ser protocolo
// binário real (não dá pra reimplementar em poucas linhas com segurança).
type amqpState struct {
	conn    *amqp.Connection
	channel *amqp.Channel

	queueName     string
	correlationID string
	replyTo       string
	tag           string
	lastErr       string
	lastStatus    float64 // 0 = sucesso, 1 = erro (sem tabela de códigos na TDN)
}

func newAMQPObject() *advplrt.ObjectValue {
	obj := advplrt.NewObject("TAMQP", nil)
	obj.Native = &amqpState{}
	obj.SetProp("CHANNELNUMBER", advplrt.NewNumber(0))
	obj.SetProp("BODY", advplrt.NewString(""))
	obj.SetProp("CONSUMETIMEOUT", advplrt.NewNumber(5))
	return obj
}

func (v *VM) callTAMQPMethod(obj *advplrt.ObjectValue, method string, args []advplrt.Value) error {
	st, ok := obj.Native.(*amqpState)
	if !ok {
		return fmt.Errorf("tAMQP: objeto sem estado interno")
	}

	fail := func(err error) {
		st.lastErr = err.Error()
		st.lastStatus = 1
	}
	ok1 := func() {
		st.lastErr = ""
		st.lastStatus = 0
	}

	switch method {
	case "NEW":
		host := getArgString(args, 0, "")
		port := int(toNumber(getArg(args, 1)))
		if port == 0 {
			port = 5672
		}
		user := getArgString(args, 2, "guest")
		pass := getArgString(args, 3, "guest")
		channelNum := toNumber(getArg(args, 4))
		vhost := getArgString(args, 5, "/")

		obj.SetProp("CHANNELNUMBER", advplrt.NewNumber(channelNum))

		uri := fmt.Sprintf("amqp://%s:%s@%s:%d", url.QueryEscape(user), url.QueryEscape(pass), host, port)
		if vhost != "" && vhost != "/" {
			uri += "/" + url.PathEscape(vhost)
		}
		conn, err := amqp.Dial(uri)
		if err != nil {
			fail(err)
			v.push(obj)
			return nil
		}
		ch, err := conn.Channel()
		if err != nil {
			fail(err)
			_ = conn.Close()
			v.push(obj)
			return nil
		}
		st.conn, st.channel = conn, ch
		ok1()
		v.push(obj)
		return nil

	case "QUEUEDECLARE":
		if st.channel == nil {
			fail(fmt.Errorf("sem conexão ativa"))
			v.push(advplrt.NewBool(false))
			return nil
		}
		name := getArgString(args, 0, "")
		durable := advplrt.ToBool(getArg(args, 1))
		exclusive := advplrt.ToBool(getArg(args, 2))
		autoDelete := advplrt.ToBool(getArg(args, 3))
		passive := advplrt.ToBool(getArg(args, 4))

		var q amqp.Queue
		var err error
		if passive {
			q, err = st.channel.QueueDeclarePassive(name, durable, autoDelete, exclusive, false, nil)
		} else {
			q, err = st.channel.QueueDeclare(name, durable, autoDelete, exclusive, false, nil)
		}
		if err != nil {
			fail(err)
			v.push(advplrt.NewBool(false))
			return nil
		}
		st.queueName = q.Name
		ok1()
		v.push(advplrt.NewBool(true))
		return nil

	case "EXCHANGEDECLARE":
		if st.channel == nil {
			fail(fmt.Errorf("sem conexão ativa"))
			v.push(advplrt.NewBool(false))
			return nil
		}
		name := getArgString(args, 0, "")
		kind := getArgString(args, 1, "direct")
		passive := advplrt.ToBool(getArg(args, 2))
		durable := advplrt.ToBool(getArg(args, 3))
		autoDelete := advplrt.ToBool(getArg(args, 4))

		var err error
		if passive {
			err = st.channel.ExchangeDeclarePassive(name, kind, durable, autoDelete, false, false, nil)
		} else {
			err = st.channel.ExchangeDeclare(name, kind, durable, autoDelete, false, false, nil)
		}
		if err != nil {
			fail(err)
			v.push(advplrt.NewBool(false))
			return nil
		}
		ok1()
		v.push(advplrt.NewBool(true))
		return nil

	case "QUEUEBIND":
		if st.channel == nil {
			fail(fmt.Errorf("sem conexão ativa"))
			v.push(advplrt.NewBool(false))
			return nil
		}
		queue := getArgString(args, 0, "")
		exchange := getArgString(args, 1, "")
		routingKey := getArgString(args, 2, "")
		if err := st.channel.QueueBind(queue, routingKey, exchange, false, nil); err != nil {
			fail(err)
			v.push(advplrt.NewBool(false))
			return nil
		}
		ok1()
		v.push(advplrt.NewBool(true))
		return nil

	case "BASICPUBLISH":
		if st.channel == nil {
			fail(fmt.Errorf("sem conexão ativa"))
			v.push(advplrt.NewBool(false))
			return nil
		}
		exchange := getArgString(args, 0, "")
		routingKey := getArgString(args, 1, "")
		persistent := advplrt.ToBool(getArg(args, 2))
		msg := getArgString(args, 3, "")
		correlationID := getArgString(args, 4, "")
		replyTo := getArgString(args, 5, "")

		mode := amqp.Transient
		if persistent {
			mode = amqp.Persistent
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := st.channel.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
			DeliveryMode:  mode,
			Body:          []byte(msg),
			CorrelationId: correlationID,
			ReplyTo:       replyTo,
		})
		if err != nil {
			fail(err)
			v.push(advplrt.NewBool(false))
			return nil
		}
		ok1()
		v.push(advplrt.NewBool(true))
		return nil

	case "BASICCONSUME":
		if st.channel == nil {
			fail(fmt.Errorf("sem conexão ativa"))
			obj.SetProp("BODY", advplrt.NewString(""))
			v.push(advplrt.NewBool(false))
			return nil
		}
		queue := getArgString(args, 0, "")
		autoAck := advplrt.ToBool(getArg(args, 1))
		waitingEvent := advplrt.ToBool(getArg(args, 2))

		timeoutSec := advplrt.ToFloat(consumeTimeoutProp(obj))
		if waitingEvent {
			timeoutSec = 30
		}
		if timeoutSec <= 0 {
			timeoutSec = 5
		}

		deliveries, err := st.channel.Consume(queue, "", autoAck, false, false, false, nil)
		if err != nil {
			fail(err)
			obj.SetProp("BODY", advplrt.NewString(""))
			v.push(advplrt.NewBool(false))
			return nil
		}
		select {
		case d, chOk := <-deliveries:
			if !chOk {
				fail(fmt.Errorf("canal de consumo fechado"))
				obj.SetProp("BODY", advplrt.NewString(""))
				v.push(advplrt.NewBool(false))
				return nil
			}
			st.queueName = queue
			st.correlationID = d.CorrelationId
			st.replyTo = d.ReplyTo
			st.tag = fmt.Sprintf("%d", d.DeliveryTag)
			obj.SetProp("BODY", advplrt.NewString(string(d.Body)))
			ok1()
			v.push(advplrt.NewBool(true))
			return nil
		case <-time.After(time.Duration(timeoutSec * float64(time.Second))):
			fail(fmt.Errorf("timeout aguardando mensagem"))
			obj.SetProp("BODY", advplrt.NewString(""))
			v.push(advplrt.NewBool(false))
			return nil
		}

	case "BASICQOS":
		if st.channel == nil {
			fail(fmt.Errorf("sem conexão ativa"))
			v.push(advplrt.NewBool(false))
			return nil
		}
		// TDN: BasicQos(nprefetchSize, nprefetchCount, bglobal) — a ordem dos
		// dois primeiros parâmetros é a do protocolo AMQP (basic.qos), mas a
		// lib amqp091-go inverte a assinatura para Qos(prefetchCount,
		// prefetchSize, global); aqui remapeamos para não trocar o efeito.
		prefetchSize := int(toNumber(getArg(args, 0)))
		prefetchCount := int(toNumber(getArg(args, 1)))
		global := advplrt.ToBool(getArg(args, 2))
		if err := st.channel.Qos(prefetchCount, prefetchSize, global); err != nil {
			fail(err)
			v.push(advplrt.NewBool(false))
			return nil
		}
		ok1()
		v.push(advplrt.NewBool(true))
		return nil

	case "BASICACK":
		if st.channel == nil {
			fail(fmt.Errorf("sem conexão ativa"))
			v.push(advplrt.NewBool(false))
			return nil
		}
		tag := uint64(toNumber(getArg(args, 0)))
		multiple := advplrt.ToBool(getArg(args, 1))
		if err := st.channel.Ack(tag, multiple); err != nil {
			fail(err)
			v.push(advplrt.NewBool(false))
			return nil
		}
		ok1()
		v.push(advplrt.NewBool(true))
		return nil

	case "CORRELATIONID":
		v.push(advplrt.NewString(st.correlationID))
		return nil
	case "REPLYTO":
		v.push(advplrt.NewString(st.replyTo))
		return nil
	case "TAG":
		v.push(advplrt.NewString(st.tag))
		return nil
	case "QUEUENAME":
		v.push(advplrt.NewString(st.queueName))
		return nil
	case "ERROR":
		v.push(advplrt.NewString(st.lastErr))
		return nil
	case "STATUS":
		v.push(advplrt.NewNumber(st.lastStatus))
		return nil

	case "MESSAGECOUNT":
		if st.channel == nil || st.queueName == "" {
			v.push(advplrt.NewNumber(0))
			return nil
		}
		q, err := st.channel.QueueDeclarePassive(st.queueName, false, false, false, false, nil)
		if err != nil {
			fail(err)
			v.push(advplrt.NewNumber(0))
			return nil
		}
		v.push(advplrt.NewNumber(float64(q.Messages)))
		return nil

	case "CONSUMERCOUNT":
		if st.channel == nil || st.queueName == "" {
			v.push(advplrt.NewNumber(0))
			return nil
		}
		q, err := st.channel.QueueDeclarePassive(st.queueName, false, false, false, false, nil)
		if err != nil {
			fail(err)
			v.push(advplrt.NewNumber(0))
			return nil
		}
		v.push(advplrt.NewNumber(float64(q.Consumers)))
		return nil

	default:
		return fmt.Errorf("unknown method %s on tAMQP", method)
	}
}

// consumeTimeoutProp lê a propriedade ConsumeTimeout diretamente de obj.Props
// (não de um campo Go em cache) porque a TDN documenta a propriedade como
// gravável (Somente Leitura: N) — o usuário pode fazer
// `oRecv:ConsumeTimeout := 10` a qualquer momento antes de BasicConsume().
func consumeTimeoutProp(obj *advplrt.ObjectValue) advplrt.Value {
	if val, ok := obj.Props["CONSUMETIMEOUT"]; ok {
		return val
	}
	return advplrt.NewNumber(5)
}
