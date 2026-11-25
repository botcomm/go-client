package botcomm

import (
	"context"
	"testing"
)

func TestArgumentCommandRouting(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, tc := range []struct {
			content, value, replyTo, command, argument string
			alwaysCommand                              bool
		}{
			{content: "/now", command: "/now", alwaysCommand: true},
			{content: "/city", command: "/city", alwaysCommand: true},
			{content: "/city \tNew  York 🙂\r\n", command: "/city", argument: "New  York 🙂"},
			{content: "/now args"}, {content: "/citywrong Warsaw"}, {content: "ordinary text"},
			{content: "A label", value: "/now", replyTo: "keyboard", alwaysCommand: true},
			{content: "A label", value: "/city Warsaw", replyTo: "keyboard"},
			{content: "/city Warsaw", replyTo: "keyboard"},
		} {
			b := newTestBot()
			b.ArgumentCommands = enabled
			b.commands = map[string]*Command{"/now": {Value: "/now"}, "/city": {Value: "/city", Input: &CommandInput{Type: "text"}}}
			msg := &Message{Content: tc.content, Value: tc.value, ReplyTo: tc.replyTo}
			if !b.route(context.Background(), msg) {
				t.Fatal("route failed")
			}

			wantCommand := tc.alwaysCommand || (enabled && tc.command != "")
			if (len(b.Commands) == 1) != wantCommand {
				t.Fatalf("enabled=%v %+v: wrong channel", enabled, tc)
			}

			if msg.Content != tc.content {
				t.Fatal("content changed")
			}

			if enabled && msg.Command != tc.command {
				t.Fatalf("command=%q, want %q", msg.Command, tc.command)
			}

			if enabled && msg.Argument != tc.argument {
				t.Fatalf("argument=%q, want %q", msg.Argument, tc.argument)
			}
		}
	}
}

func TestCollectionRequestDoesNotBecomeMessage(t *testing.T) {
	b := newTestBot()
	b.CollectionRequests = make(chan *CollectionRequest, 1)
	msg := b.dispatch([]byte(`{"type":"collection_request","data":{"id":"r","sessionID":"s","key":"channels","action":"remove","itemID":"1","revision":3,"expiresTS":"2026-10-09T20:00:00Z"}}`))
	if msg != nil || len(b.Messages) != 0 || len(b.Commands) != 0 {
		t.Fatal("configuration routed as a message")
	}

	select {
	case req := <-b.CollectionRequests:
		if req.SessionID != "s" || req.ItemID != "1" || req.Revision == nil || *req.Revision != 3 || req.ExpiresTS.IsZero() {
			t.Fatalf("request: %+v", req)
		}
	default:
		t.Fatal("missing collection request")
	}
}
