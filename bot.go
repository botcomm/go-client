// Package botcomm is the official Go SDK for building BotComm bots.
//
// A bot authenticates with its client credentials (client_credentials grant,
// re-granted automatically before the access token expires), keeps an SSE
// stream to /updates and receives incoming messages on the Messages and
// Commands channels. On every (re)connect the SDK back-fills unread messages
// via the REST API, so nothing is lost while the bot was offline. Replies go
// through Send/SendKeyboard, and Broadcast reaches every session at once. Typing, Edit and Delete update the
// conversation in place. RegisterSettings declares per-session settings the
// user edits in the app; changes arrive on the Settings channel. Rate-limited
// (429) requests are retried once after Retry-After. See examples/echo for a
// minimal bot.
package botcomm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	apiHost  = "https://api.botcomm.app"   // resource API
	authHost = "https://oauth.botcomm.app" // client_credentials token endpoint
)

const (
	chanBuffer       = 64               // Messages/Commands channel buffer
	reconnectDelay   = 5 * time.Second  // pause between reconnect attempts
	regrantRetry     = 10 * time.Second // retry interval after a failed re-grant
	regrantEarliness = time.Minute      // re-grant this long before expiry
	readWait         = 90 * time.Second // read deadline, reset by every received line
	httpTimeout      = time.Minute      // timeout for REST API requests
)

// Bot is a BotComm bot: an authenticated client connected to the /updates
// event stream. Configure the exported fields before calling Run (or use New).
type Bot struct {
	ArgumentCommands bool       // opt in before Run to route declared command input
	StartupCommands  []*Command // published before connecting; nil leaves commands unchanged
	InputMode        string     // optional mode to publish after StartupCommands

	ID string // bot ID, resolved from the access token subject on Run

	ClientID     string
	ClientSecret string

	Messages           chan *Message           // incoming messages that are not registered commands
	Commands           chan *Message           // incoming messages matching a registered command value
	Settings           chan *SessionSettings   // per-session settings after a change (dropped when nobody reads)
	Reactions          chan *Message           // reaction snapshots; dropped when nobody reads
	ReadReceipts       chan *ReadReceipt       // receipts for the bot's messages (dropped when nobody reads)
	CollectionRequests chan *CollectionRequest // native list/add/remove operations; reply with RespondCollection

	// NewSession is called when a user opens a new session with the bot.
	// Set it before calling Run.
	NewSession func(session *Session)

	acceptMessage func(context.Context, *Message) error

	host, authHost string // set on Run; tests point them at a local server
	commands       map[string]*Command
	backfilled     map[string]struct{} // message IDs delivered by the latest backfill, pruned by dispatch

	mu      sync.RWMutex // guards tk, conn, commands and backfilled
	tk      *token
	conn    io.ReadCloser
	cancel  context.CancelFunc
	stopped atomic.Bool

	httpClient *http.Client
}

// New creates a bot and runs it: authorizes with the client credentials,
// connects to the /updates stream and starts listening. The bot stops when
// ctx is cancelled or Stop is called. Supply at most one optional Opts value.
func New(ctx context.Context, clientID, clientSecret string, opts ...*Opts) (*Bot, error) {
	b := &Bot{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
	if err := b.Run(ctx, opts...); err != nil {
		return nil, err
	}

	return b, nil
}

// Run authorizes the bot, connects to the event stream and starts the listen
// loop. It returns an error if the initial grant or connect fails; after that
// the SDK keeps the token fresh and reconnects on its own. Supply at most one
// optional Opts value; a non-nil value replaces the bot's startup fields.
// Setup runs before connecting and stops at the first error without rolling
// back earlier server updates. Reconnects do not republish configuration.
func (b *Bot) Run(ctx context.Context, opts ...*Opts) error {
	if len(opts) > 1 {
		return errors.New("expected at most one Opts value")
	}

	options := Opts{
		Commands:         b.StartupCommands,
		InputMode:        b.InputMode,
		ArgumentCommands: b.ArgumentCommands,
		NewSession:       b.NewSession,
	}
	if len(opts) == 1 && opts[0] != nil {
		options = *opts[0]
		b.StartupCommands = options.Commands
		b.InputMode = options.InputMode
		b.ArgumentCommands = options.ArgumentCommands
		b.NewSession = options.NewSession
	}

	ctx, cancel := context.WithCancel(ctx)
	b.mu.Lock()
	b.cancel = cancel
	b.mu.Unlock()
	if b.host == "" {
		b.host, b.authHost = apiHost, authHost
	}

	if b.Messages == nil {
		b.Messages = make(chan *Message, chanBuffer)
	}

	if b.Commands == nil {
		b.Commands = make(chan *Message, chanBuffer)
	}

	if b.CollectionRequests == nil {
		b.CollectionRequests = make(chan *CollectionRequest, chanBuffer)
	}

	if b.Settings == nil {
		b.Settings = make(chan *SessionSettings, chanBuffer)
	}

	if b.Reactions == nil {
		b.Reactions = make(chan *Message, chanBuffer)
	}

	if b.ReadReceipts == nil {
		b.ReadReceipts = make(chan *ReadReceipt, chanBuffer)
	}

	if b.commands == nil {
		b.commands = make(map[string]*Command)
	}

	if b.httpClient == nil {
		b.httpClient = &http.Client{Timeout: httpTimeout}
	}

	if err := b.authorize(ctx); err != nil {
		cancel()
		return fmt.Errorf("failed to authorize: %w", err)
	}

	if b.StartupCommands != nil {
		if err := b.RegisterCommands(ctx, b.StartupCommands); err != nil {
			cancel()
			return err
		}
	}

	if options.Settings != nil {
		if err := b.RegisterSettings(ctx, options.Settings); err != nil {
			cancel()
			return err
		}
	}

	if options.Collections != nil {
		if err := b.RegisterCollections(ctx, options.Collections); err != nil {
			cancel()
			return err
		}
	}

	if options.OnlineStatusVisible != nil {
		if err := b.SetOnlineStatusVisible(ctx, *options.OnlineStatusVisible); err != nil {
			cancel()
			return err
		}
	}

	if b.InputMode != "" {
		if err := b.SetInputMode(ctx, b.InputMode); err != nil {
			cancel()
			return err
		}
	}

	if err := b.connect(); err != nil {
		cancel()
		return fmt.Errorf("failed to connect: %w", err)
	}

	go b.listen(ctx)

	return nil
}

// Stop cancels recovery, re-granting, and the event stream.
func (b *Bot) Stop() {
	b.stopped.Store(true)
	b.mu.RLock()
	conn := b.conn
	cancel := b.cancel
	b.mu.RUnlock()
	if cancel != nil {
		cancel()
	}

	if conn != nil {
		_ = conn.Close()
	}
}

// SetOnlineStatusVisible controls whether apps show the bot's connection status.
func (b *Bot) SetOnlineStatusVisible(ctx context.Context, visible bool) error {
	return b.do(ctx, http.MethodPost, fmt.Sprintf("/bot/%s", b.ID), map[string]bool{"onlineStatusVisible": visible}, nil)
}
