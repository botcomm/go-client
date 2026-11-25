// A minimal BotComm bot that echoes every message back to the sender and
// exposes one setting.
//
// Configuration (environment):
//
//	CLIENT_ID     - bot client id (shown once on bot creation)
//	CLIENT_SECRET - bot client secret (shown once on bot creation)
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/botcomm/go-client"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("[ERROR] CLIENT_ID and CLIENT_SECRET are required")
	}

	b := &botcomm.Bot{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		InputMode:    "messages",
	}
	b.NewSession = func(session *botcomm.Session) {
		log.Printf("[INFO] new session %s", session.ID)
		if _, err := b.Send(ctx, session.ID, "Hello! I repeat everything you say."); err != nil {
			log.Printf("[ERROR] failed to greet: %v", err)
		}
	}

	if err := b.Run(ctx); err != nil {
		log.Fatalf("[ERROR] failed to start bot: %v", err)
	}

	defer b.Stop()

	if err := b.RegisterCommands(ctx, []*botcomm.Command{{Name: "Ping", Value: "/ping", Description: "Check the bot is alive"}}); err != nil {
		log.Printf("[ERROR] failed to register commands: %v", err)
	}

	if err := b.RegisterSettings(ctx, []botcomm.SettingField{{Key: "shout", Label: "Shout back", Type: "toggle"}}); err != nil {
		log.Printf("[ERROR] failed to register settings: %v", err)
	}

	log.Printf("[INFO] echo bot %s is running", b.ID)

	for {
		select {
		case msg := <-b.Messages:
			_ = b.Typing(ctx, msg.SessionID)
			if reply, err := b.Send(ctx, msg.SessionID, msg.Content); err != nil {
				log.Printf("[ERROR] failed to reply: %v", err)
			} else {
				log.Printf("[INFO] echoed as %s", reply.ID)
			}
		case msg := <-b.Commands:
			if _, err := b.Send(ctx, msg.SessionID, "pong"); err != nil {
				log.Printf("[ERROR] failed to reply: %v", err)
			}
		case s := <-b.Settings:
			log.Printf("[INFO] session %s settings: %v", s.SessionID, s.Settings)
		case <-ctx.Done():
			return
		}
	}
}
