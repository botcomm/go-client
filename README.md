# BotComm Go SDK

Build bots with `github.com/botcomm/go-client` (package `botcomm`). See [the echo example](examples/echo/main.go) for credentials, lifecycle, and message handling.

`ReadReceipts` receives `ReadReceipt` events (`SessionID`, `MessageID`, `By`) when a user reads a bot message. Consume this buffered channel to react to reads; receipts are dropped when the buffer is full and are not replayed after reconnecting. `FeaturedBots(ctx)` returns the operator's ordered list of public bots.

## Quickstart

Create a bot in the [BotComm console](https://web.botcomm.app/console/bots) and save its credentials. In the same terminal where you'll run the bot, set:

```sh
export CLIENT_ID='your-client-id'
export CLIENT_SECRET='your-client-secret'
```

In an empty directory, initialize a module and add the SDK (Go 1.23 or later):

```sh
go mod init example.com/echo
go get github.com/botcomm/go-client
```

Save this as `main.go`:

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/botcomm/go-client"
)

func main() {
	ctx := context.Background()
	bot, err := botcomm.New(ctx, os.Getenv("CLIENT_ID"), os.Getenv("CLIENT_SECRET"), &botcomm.Opts{InputMode: "messages"})
	if err != nil {
		log.Fatal(err)
	}
	defer bot.Stop()

	for msg := range bot.Messages {
		if _, err := bot.Send(ctx, msg.SessionID, msg.Content); err != nil {
			log.Print(err)
		}
	}
}
```

Run `go run .`, open your bot’s chat in BotComm, and send `hello`. The bot should reply with `hello`.

## Source layout

The SDK uses one `botcomm` package. Each feature keeps its types and methods together:

- `bot.go` and `options.go`: bot lifecycle and startup configuration.
- `message.go`, `card.go` and `broadcast.go`: messages, keyboards, streams, reactions, cards and broadcasts.
- `command.go` and `session.go`: command registration and routing, sessions, settings and receipts.
- `attachment.go`, `collection.go` and `featured.go`: attachments, session collections and featured bots.
- `auth.go` and `http.go`: authentication, token renewal and HTTP requests.
- `updates.go` and `recovery.go`: event streaming, reconnects and unread recovery.

Tests live alongside their features in `*_test.go` files.

## Startup options

Pass an optional `*botcomm.Opts` to `New`. Setters update the same options value and return its pointer for chaining:

```go
opts := &botcomm.Opts{}
opts.SetInputMode("commands").SetOnlineStatusVisible(true)
opts.SetCommands([]*botcomm.Command{
    {Name: "Help", Value: "/help", Description: "Show available commands"},
})

b, err := botcomm.New(ctx, clientID, clientSecret, opts)
if err != nil {
    return err
}
defer b.Stop()
```

Options also expose fields for struct literals, such as `&botcomm.Opts{InputMode: "messages"}`. Additional setters are `SetSettings`, `SetCollections`, `SetArgumentCommands` and `SetNewSession`. Configure the callback before startup so it can receive the first session event.

Nil slices, an empty input mode and nil `OnlineStatusVisible` preserve stored configuration. Non-nil empty slices clear commands, settings or collections. `SetOnlineStatusVisible(false)` explicitly hides the indicator; `ArgumentCommands` defaults to false.

Startup authenticates, publishes commands, settings and collections, applies visibility and input mode, then connects and recovers unread messages. A setup error stops startup and is returned to the caller; earlier server updates remain applied. Reconnects do not republish configuration.

Existing calls without options still work. Manual construction also supports `b.Run(ctx, opts)`: a non-nil options value replaces the existing `StartupCommands`, `InputMode`, `ArgumentCommands` and `NewSession` fields for that startup. Omitted or nil options keep those fields. Both `New` and `Run` accept at most one options value.

## Commands-only input

Configure before `Run` so unread recovery uses the same routing as live messages:

```go
b := &botcomm.Bot{
    ClientID: clientID, ClientSecret: clientSecret,
    ArgumentCommands: true,
    InputMode: "commands",
    StartupCommands: []*botcomm.Command{
        {Name: "Set city", Value: "/city", Input: &botcomm.CommandInput{
            Type: "text", Label: "City", Placeholder: "Warsaw", Required: true,
        }},
    },
}
if err := b.Run(ctx); err != nil { return err }
// Read b.Commands: msg.Command == "/city", msg.Argument == "New York".
// msg.Content is still "/city New York".
```

`SetInputMode(ctx, "messages")` enables ordinary messages. `RegisterCommands`
can update definitions at runtime.


## Online status visibility

After `Run`, use `SetOnlineStatusVisible(ctx, true)` to show the bot's online/offline indicator in the web and iOS apps, or `false` to hide it. Check the returned error. The setting defaults to hidden and persists between restarts.

## Receive-only mode and collections

Use input mode `none` for a receive-only chat. The composer is hidden. Users can still answer buttons, respond to cards, react to messages, and change session settings. Publish native session collections with `RegisterCollections`, `CollectionRequests`, and `RespondCollection`. Register the schema before enabling `none`. Persist and atomically compare/increment list revisions on mutations, reject expired requests, and return results outside chat history.

## Photos and albums

Upload each image with `UploadAttachment(ctx, sessionID, name, data)`, then pass the returned IDs in `SendOptions.Attachments` to `SendWithOptions`. Up to ten attachments share one message and its caption; content may be empty. Uploads belong to one session and must be attached within ten minutes. Images are limited to 5 MB each. Messages decode attachment metadata in `Message.Attachments`.

## Message metadata

`Message`, `MessageInput`, `MessageUpdate`, `SendOptions`, `EditOptions`, and `Broadcast` expose `Metadata` as `json.RawMessage`. Supply a JSON object up to 4096 compact UTF-8 bytes. For edits, nil preserves it and `json.RawMessage("null")` removes it.

`UpdateCard` also accepts `CardUpdate.Metadata` and sends it as message metadata alongside the card update, with the same preserve, replace, and remove behavior.

## Streaming replies and reactions

`StartStream(ctx, sessionID)` creates an empty live bubble. Call `UpdateStream(ctx, sessionID, messageID, StreamInput{Content: text, Sequence: n, Status: "active"})` with accumulated text and increasing sequence numbers, batching at about 500ms. Finish with `"completed"` or `"failed"`. Identical snapshot retries are safe; idle streams expire after two minutes, so send unchanged heartbeats while waiting for a provider. `SendWithOptions` also accepts `SendOptions{Stream: true}`.

`React(ctx, sessionID, messageID, "❤️")` replaces the bot's reaction; `""` removes it. Read full message snapshots from `bot.Reactions` for live reaction changes. The queue is bounded; fetch the message after reconnect to reconcile old reactions. Reactions are separate from `Messages` and `Commands`.


## Agent cards

Use `SendMessage` with `MessageInput.Card` and `UpdateMessage` with
`MessageUpdate.Card`. Go uses `AgentCard` and `CardUpdate`; JSON fields
`parentID`, `sourceID`, `planID` and `url` map to
`ParentID`, `SourceID`, `PlanID` and `URL`.

Set `MessageInput.IdempotencyKey` to a stable value such as `"themis:issue:" + issueNodeID` when sending the first card. Retrying with the same key in that session returns the saved message. Use `RecentMessages(ctx, sessionID)` to read up to 10 messages from the conversation, newest first.

Set `AgentCard.Kind` using `CardKindPlan`, `CardKindApproval`,
`CardKindForm` or `CardKindArtifact`. Convert existing string variables with
`CardKind(value)`.

Tasks use `CardTask.Status` with the `CardTaskStatus` type and
constants `CardTaskStatusQueued`, `CardTaskStatusRunning`, `CardTaskStatusWaiting`,
`CardTaskStatusCompleted`, `CardTaskStatusFailed` and `CardTaskStatusCancelled`.
For example: `CardTask{ID: "review", Title: "Review changes", Status: CardTaskStatusQueued}`.
Message and card send/update methods reject empty or unknown task statuses before
making a request. Convert existing string variables with `CardTaskStatus(value)`;
string literals remain supported.

| Kind | Initial status | Other statuses |
|---|---|---|
| `plan` | `queued` | `running`, `waiting`, `completed`, `failed`, `cancelled` |
| `approval` | `pending` | `approved`, `rejected`, `expired`, `cancelled` |
| `form` | `pending` | `submitted`, `expired`, `cancelled` |
| `artifact` | `completed` | None; immutable |

The server sets the initial status and revision. Plan updates require
the latest `revision` and a status other than `queued`; completed, failed and
cancelled plans cannot be reopened. Stale revisions return 409. Approval/form
updates only cancel pending, unexpired requests; answers and expiration determine
their other statuses. Artifact cards cannot be updated.

An artifact requires an HTTP(S) `url` without credentials and may include
`parentID`, the message ID of an existing plan from the same bot in the same chat.

`sourceID` is different: only plans accept it, pointing to an approved approval
or submitted form from the same bot and chat. Each source permits one plan.
The server sets `planID` on that source; never supply `planID` yourself.
Links cannot be edited, and linking cards does not execute work.

A plan can contain up to 50 tasks. Updating its task list replaces the full list.
