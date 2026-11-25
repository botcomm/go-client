package botcomm

import (
	"context"
	"testing"
)

func TestDispatchCommandRouting(t *testing.T) {
	b := newTestBot()
	b.commands["/start"] = &Command{Name: "Start", Value: "/start"}

	msg := b.dispatch([]byte(`{"type":"message","data":{"id":"m2","sessionID":"s1","content":"/start"}}`))
	if msg == nil {
		t.Fatal("expected dispatch to return the routed message")
	}

	select {
	case got := <-b.Commands:
		if got.ID != "m2" || got.Content != "/start" {
			t.Fatalf("unexpected command message: %+v", got)
		}
	default:
		t.Fatal("expected the command on the Commands channel")
	}

	if len(b.Messages) != 0 {
		t.Fatal("command message must not reach the Messages channel")
	}

	// content not matching a registered command goes to Messages
	if b.dispatch([]byte(`{"type":"message","data":{"id":"m3","sessionID":"s1","content":"/stop"}}`)) == nil {
		t.Fatal("expected dispatch to return the routed message")
	}

	select {
	case got := <-b.Messages:
		if got.ID != "m3" {
			t.Fatalf("unexpected message: %+v", got)
		}
	default:
		t.Fatal("expected the unregistered command on the Messages channel")
	}
}

func TestDispatchKeyboardAnswerRouting(t *testing.T) {
	b := newTestBot()
	b.commands["/start"] = &Command{Name: "Start", Value: "/start"}

	// a keyboard answer routes by its machine value, not the label content
	frame := []byte(`{"type":"message","data":{"id":"m4","sessionID":"s1","content":"Yes 👍",` +
		`"replyTo":"m1","value":"confirm_yes"}}`)
	if b.dispatch(frame) == nil {
		t.Fatal("expected dispatch to return the routed message")
	}

	select {
	case got := <-b.Messages:
		if got.ReplyTo != "m1" || got.Value != "confirm_yes" || got.Content != "Yes 👍" {
			t.Fatalf("unexpected answer message: %+v", got)
		}
	default:
		t.Fatal("expected the answer on the Messages channel")
	}

	// an answer whose value matches a registered command goes to Commands
	frame = []byte(`{"type":"message","data":{"id":"m5","sessionID":"s1","content":"Start over",` +
		`"replyTo":"m1","value":"/start"}}`)
	if b.dispatch(frame) == nil {
		t.Fatal("expected dispatch to return the routed message")
	}

	select {
	case got := <-b.Commands:
		if got.ID != "m5" || got.Value != "/start" {
			t.Fatalf("unexpected command answer: %+v", got)
		}
	default:
		t.Fatal("expected the command-valued answer on the Commands channel")
	}
}

func TestDurableAcceptancePrecedesDelivery(t *testing.T) {
	bot := newTestBot()
	accepted := false
	bot.SetMessageAcceptor(func(ctx context.Context, msg *Message) error {
		if len(bot.Messages) != 0 {
			t.Fatal("message delivered before acceptance")
		}

		accepted = true

		return nil
	})
	message := bot.dispatchContext(context.Background(), []byte(`{"type":"message","data":{"id":"durable","sessionID":"session","content":"hello"}}`))
	if !accepted || message == nil || len(bot.Messages) != 1 {
		t.Fatal("durable acceptance was skipped")
	}
}

func TestFailedAcceptanceIsNotDeliveredOrAcknowledged(t *testing.T) {
	bot := newTestBot()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bot.SetMessageAcceptor(func(context.Context, *Message) error { cancel(); return context.Canceled })
	message := bot.dispatchContext(ctx, []byte(`{"type":"message","data":{"id":"durable","sessionID":"session","content":"hello"}}`))
	if message != nil || len(bot.Messages) != 0 {
		t.Fatal("failed persistence allowed acknowledgment")
	}
}
