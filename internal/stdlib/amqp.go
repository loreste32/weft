//go:build !js && !slim

package stdlib

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/loreste/weft/internal/runtime"
)

// packageAMQP — RabbitMQ / AMQP 0-9-1.
//
//	ch := amqp.connect("amqp://guest:guest@localhost:5672/")?
//	ch.publish("", "queue", body)?
//	ch.consume("queue", fn(msg) { ... })?
func packageAMQP(env *runtime.Env) runtime.Value {
	p := pkg()

	set(p, "connect", func(args []runtime.Value) (runtime.Value, error) {
		url := "amqp://guest:guest@127.0.0.1:5672/"
		if len(args) >= 1 && args[0].String() != "" {
			url = args[0].String()
		}
		conn, err := amqp.Dial(url)
		if err != nil {
			return errRes(err.Error(), "amqp"), nil
		}
		ch, err := conn.Channel()
		if err != nil {
			_ = conn.Close()
			return errRes(err.Error(), "amqp"), nil
		}
		return runtime.Ok(wrapAMQP(env, conn, ch)), nil
	}, 1)

	return p
}

func wrapAMQP(env *runtime.Env, conn *amqp.Connection, ch *amqp.Channel) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(name string, arity int, fn runtime.Builtin) {
		mo.Keys = append(mo.Keys, name)
		mo.Vals[name] = runtime.MakeBuiltin("amqp."+name, arity, fn)
	}

	// ch.queue_declare(name, opts?) -> Result[{name, messages, consumers}]
	put("queue_declare", 2, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("ch.queue_declare(name)", "amqp"), nil
		}
		name := args[0].String()
		durable, autoDelete, exclusive := false, false, false
		if len(args) >= 2 {
			if b, ok := mapGet(args[1], "durable"); ok && b.Kind == runtime.KindBool {
				durable = b.B
			}
			if b, ok := mapGet(args[1], "auto_delete"); ok && b.Kind == runtime.KindBool {
				autoDelete = b.B
			}
		}
		q, err := ch.QueueDeclare(name, durable, autoDelete, exclusive, false, nil)
		if err != nil {
			return errRes(err.Error(), "amqp"), nil
		}
		out := runtime.NewMap()
		omo := out.Obj.(*runtime.MapObj)
		omo.Keys = []string{"name", "messages", "consumers"}
		omo.Vals["name"] = runtime.Str(q.Name)
		omo.Vals["messages"] = runtime.Int(int64(q.Messages))
		omo.Vals["consumers"] = runtime.Int(int64(q.Consumers))
		return runtime.Ok(out), nil
	})

	// ch.exchange_declare(name, kind, opts?)
	put("exchange_declare", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("ch.exchange_declare(name, kind)", "amqp"), nil
		}
		durable := true
		if len(args) >= 3 {
			if b, ok := mapGet(args[2], "durable"); ok && b.Kind == runtime.KindBool {
				durable = b.B
			}
		}
		if err := ch.ExchangeDeclare(args[0].String(), args[1].String(), durable, false, false, false, nil); err != nil {
			return errRes(err.Error(), "amqp"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	// ch.bind(queue, exchange, key)
	put("bind", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("ch.bind(queue, exchange, key)", "amqp"), nil
		}
		if err := ch.QueueBind(args[0].String(), args[2].String(), args[1].String(), false, nil); err != nil {
			return errRes(err.Error(), "amqp"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	// ch.publish(exchange, key, body, opts?)
	put("publish", 4, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 3 {
			return errRes("ch.publish(exchange, routing_key, body)", "amqp"), nil
		}
		contentType := "text/plain"
		if len(args) >= 4 {
			if s := mapGetStr(args[3], "content_type", ""); s != "" {
				contentType = s
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := ch.PublishWithContext(ctx,
			args[0].String(),
			args[1].String(),
			false, false,
			amqp.Publishing{
				ContentType:  contentType,
				Body:         []byte(args[2].String()),
				DeliveryMode: amqp.Persistent,
				Timestamp:    time.Now(),
			},
		)
		if err != nil {
			return errRes(err.Error(), "amqp"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	})

	// ch.consume(queue, handler, opts?) -> Result
	// starts consumers in background; handler(msg) with msg.data, msg.ack(), msg.nack()
	put("consume", 3, func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("ch.consume(queue, handler)", "amqp"), nil
		}
		queue := args[0].String()
		handler := args[1]
		if handler.Kind != runtime.KindFunc && handler.Kind != runtime.KindBuiltin {
			return errRes("handler must be a function", "amqp"), nil
		}
		if env.Call == nil {
			return errRes("runtime Call not configured", "amqp"), nil
		}
		autoAck := false
		if len(args) >= 3 {
			if b, ok := mapGet(args[2], "auto_ack"); ok && b.Kind == runtime.KindBool {
				autoAck = b.B
			}
		}
		deliveries, err := ch.Consume(queue, "", autoAck, false, false, false, nil)
		if err != nil {
			return errRes(err.Error(), "amqp"), nil
		}
		go func() {
			for d := range deliveries {
				msg := amqpMsgValue(&d, autoAck)
				_, _ = env.Call(handler, []runtime.Value{msg})
			}
		}()
		return runtime.Ok(runtime.Unit()), nil
	})

	put("close", 0, func(args []runtime.Value) (runtime.Value, error) {
		_ = ch.Close()
		_ = conn.Close()
		return runtime.Ok(runtime.Unit()), nil
	})

	return m
}

func amqpMsgValue(d *amqp.Delivery, autoAck bool) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(k string, v runtime.Value) {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	put("data", runtime.Str(string(d.Body)))
	put("exchange", runtime.Str(d.Exchange))
	put("routing_key", runtime.Str(d.RoutingKey))
	put("content_type", runtime.Str(d.ContentType))
	put("ack", runtime.MakeBuiltin("msg.ack", 0, func(args []runtime.Value) (runtime.Value, error) {
		if autoAck {
			return runtime.Ok(runtime.Unit()), nil
		}
		if err := d.Ack(false); err != nil {
			return errRes(err.Error(), "amqp"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}))
	put("nack", runtime.MakeBuiltin("msg.nack", 1, func(args []runtime.Value) (runtime.Value, error) {
		requeue := true
		if len(args) >= 1 && args[0].Kind == runtime.KindBool {
			requeue = args[0].B
		}
		if err := d.Nack(false, requeue); err != nil {
			return errRes(err.Error(), "amqp"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}))
	return m
}
