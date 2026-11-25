package botcomm

import "testing"

func TestDispatchReadReceipt(t *testing.T) {
	b := newTestBot()
	b.ReadReceipts = make(chan *ReadReceipt, 1)
	frame := []byte(`{"type":"read","data":{"sessionID":"s1","messageID":"m1","by":"u1"}}`)
	if msg := b.dispatch(frame); msg != nil {
		t.Fatal("read receipts must not be confirmed as messages")
	}

	select {
	case got := <-b.ReadReceipts:
		if got.SessionID != "s1" || got.MessageID != "m1" || got.By != "u1" {
			t.Fatalf("unexpected receipt: %+v", got)
		}
	default:
		t.Fatal("expected a read receipt")
	}

	if len(b.Messages) != 0 || len(b.Commands) != 0 {
		t.Fatal("read receipts must not reach message channels")
	}
}

func TestDispatchReadReceiptWithoutConsumer(t *testing.T) {
	b := newTestBot()
	frame := []byte(`{"type":"read","data":{"sessionID":"s1","messageID":"m1"}}`)
	b.dispatch(frame) // nil channel must not block the event stream
	b.ReadReceipts = make(chan *ReadReceipt, 1)
	b.dispatch(frame)
	b.dispatch(frame) // full channel must not block the event stream
	if len(b.ReadReceipts) != 1 {
		t.Fatal("expected the first receipt to stay buffered")
	}
}

func TestDispatchMalformedReadReceipt(t *testing.T) {
	b := newTestBot()
	b.ReadReceipts = make(chan *ReadReceipt, 1)
	b.dispatch([]byte(`{"type":"read","data":{"sessionID":42}}`))
	if len(b.ReadReceipts) != 0 {
		t.Fatal("malformed receipt must not be delivered")
	}
}
