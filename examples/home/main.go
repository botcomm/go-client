// A self-hosted smart-home demo bot: the template for a bot that runs on your
// own machine and controls things only you can reach. State is in memory and
// pretend — swap the setters for real device calls.
//
// Configuration (environment):
//
//	CLIENT_ID     - bot client id (shown once on bot creation)
//	CLIENT_SECRET - bot client secret (shown once on bot creation)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/botcomm/go-client"
)

type home struct {
	Lights bool
	Heat   int
	Locked bool
}

func (h home) String() string {
	lights, door := "off", "unlocked"
	if h.Lights {
		lights = "on"
	}

	if h.Locked {
		door = "locked"
	}

	return fmt.Sprintf("💡 Lights %s\n🌡 Thermostat %d°C\n🚪 Door %s", lights, h.Heat, door)
}

var (
	mu     sync.Mutex
	states = map[string]*home{}
)

func state(sessionID string) *home {
	mu.Lock()
	defer mu.Unlock()
	if states[sessionID] == nil {
		states[sessionID] = &home{Heat: 21, Locked: true}
	}

	return states[sessionID]
}

var keyboards = map[string]struct {
	text    string
	options []botcomm.KeyboardOption
}{
	"/lights": {
		"Lights?",
		[]botcomm.KeyboardOption{
			{Label: "On", Value: "lights:on"},
			{Label: "Off", Value: "lights:off"},
		},
	},
	"/heat": {
		"Set the thermostat to",
		[]botcomm.KeyboardOption{
			{Label: "18°C", Value: "heat:18"},
			{Label: "21°C", Value: "heat:21"},
			{Label: "24°C", Value: "heat:24"},
		},
	},
	"/door": {
		"Door?",
		[]botcomm.KeyboardOption{
			{Label: "Lock", Value: "door:lock"},
			{Label: "Unlock", Value: "door:unlock"},
		},
	},
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	clientID, clientSecret := os.Getenv("CLIENT_ID"), os.Getenv("CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("[ERROR] CLIENT_ID and CLIENT_SECRET are required")
	}

	b := &botcomm.Bot{ClientID: clientID, ClientSecret: clientSecret}
	send := func(sessionID, text string) {
		if _, err := b.Send(ctx, sessionID, text); err != nil {
			log.Printf("[ERROR] send: %v", err)
		}
	}
	b.NewSession = func(session *botcomm.Session) {
		send(session.ID, "Welcome home. /status shows everything, /lights, /heat and /door control it.")
	}

	if err := b.Run(ctx); err != nil {
		log.Fatalf("[ERROR] failed to start bot: %v", err)
	}

	defer b.Stop()

	if err := b.RegisterCommands(ctx, []*botcomm.Command{
		{Name: "Status", Value: "/status", Description: "Lights, thermostat and door"},
		{Name: "Lights", Value: "/lights", Description: "Turn the lights on or off"},
		{Name: "Heat", Value: "/heat", Description: "Set the thermostat"},
		{Name: "Door", Value: "/door", Description: "Lock or unlock the door"},
	}); err != nil {
		log.Printf("[ERROR] failed to register commands: %v", err)
	}

	log.Printf("[INFO] home bot %s is running", b.ID)

	for {
		select {
		case msg := <-b.Commands:
			if msg.Content == "/status" {
				send(msg.SessionID, state(msg.SessionID).String())
				continue
			}

			kb := keyboards[msg.Content]
			if _, err := b.SendKeyboard(ctx, msg.SessionID, kb.text, kb.options); err != nil {
				log.Printf("[ERROR] send keyboard: %v", err)
			}
		case msg := <-b.Messages:
			if msg.ReplyTo == "" {
				send(msg.SessionID, "Try /status, /lights, /heat or /door.")
				continue
			}

			h := state(msg.SessionID)
			device, value, _ := strings.Cut(msg.Value, ":")
			mu.Lock()
			switch device {
			case "lights":
				h.Lights = value == "on"
			case "heat":
				fmt.Sscan(value, &h.Heat)
			case "door":
				h.Locked = value == "lock"
			}

			mu.Unlock()
			send(msg.SessionID, "Done.\n\n"+h.String())
		case <-ctx.Done():
			return
		}
	}
}
