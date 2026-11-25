package botcomm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CardMessage is the legacy wrapper; new integrations use Message directly.
type CardMessage struct{ Message }

type CardDetail struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Action struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Role  string `json:"role,omitempty"`
}

type CardAction = Action

type FormField struct {
	Key         string           `json:"key"`
	Label       string           `json:"label"`
	Type        string           `json:"type"`
	Required    bool             `json:"required,omitempty"`
	Placeholder string           `json:"placeholder,omitempty"`
	MaxLength   int              `json:"maxLength,omitempty"`
	Options     []KeyboardOption `json:"options,omitempty"`
}

type CardTaskStatus string

const (
	CardTaskStatusQueued    CardTaskStatus = "queued"
	CardTaskStatusRunning   CardTaskStatus = "running"
	CardTaskStatusWaiting   CardTaskStatus = "waiting"
	CardTaskStatusCompleted CardTaskStatus = "completed"
	CardTaskStatusFailed    CardTaskStatus = "failed"
	CardTaskStatusCancelled CardTaskStatus = "cancelled"
)

type CardTask struct {
	ID     string         `json:"id"`
	Title  string         `json:"title"`
	Status CardTaskStatus `json:"status"`
	URL    string         `json:"url,omitempty"`
}

type CardKind string

const (
	CardKindPlan     CardKind = "plan"
	CardKindApproval CardKind = "approval"
	CardKindForm     CardKind = "form"
	CardKindArtifact CardKind = "artifact"
)

type AgentCard struct {
	ParentID  string       `json:"parentID,omitempty"`
	URL       string       `json:"url,omitempty"`
	Tasks     []CardTask   `json:"tasks,omitempty"`
	SourceID  string       `json:"sourceID,omitempty"`
	PlanID    string       `json:"planID,omitempty"`
	Kind      CardKind     `json:"kind"`
	Title     string       `json:"title"`
	Summary   string       `json:"summary"`
	Details   []CardDetail `json:"details,omitempty"`
	Actions   []CardAction `json:"actions,omitempty"`
	Fields    []FormField  `json:"fields,omitempty"`
	Status    string       `json:"status"`
	Revision  int64        `json:"revision"`
	ExpiresTS *time.Time   `json:"expiresTS,omitempty"`
	Progress  *int         `json:"progress,omitempty"`
	Result    string       `json:"result,omitempty"`
	AnswerID  string       `json:"answerID,omitempty"`
}

type CardUpdate struct {
	Metadata json.RawMessage `json:"-"`
	Tasks    *[]CardTask     `json:"tasks,omitempty"`
	Revision int64           `json:"revision"`
	Status   string          `json:"status"`
	Progress *int            `json:"progress,omitempty"`
	Result   *string         `json:"result,omitempty"`
}

func validateCardTaskStatuses(tasks []CardTask) error {
	for i, item := range tasks {
		switch item.Status {
		case CardTaskStatusQueued, CardTaskStatusRunning, CardTaskStatusWaiting, CardTaskStatusCompleted, CardTaskStatusFailed, CardTaskStatusCancelled:
		default:
			return fmt.Errorf("invalid task status %q for task %d (%q)", item.Status, i, item.ID)
		}
	}

	return nil
}

func (b *Bot) SendCard(ctx context.Context, sessionID string, card AgentCard) (*CardMessage, error) {
	message, err := b.SendMessage(ctx, sessionID, MessageInput{Card: &card})
	if err != nil {
		return nil, err
	}

	return &CardMessage{Message: *message}, nil
}

func (b *Bot) UpdateCard(ctx context.Context, sessionID, messageID string, change CardUpdate) (*CardMessage, error) {
	message, err := b.UpdateMessage(ctx, sessionID, messageID, MessageUpdate{Card: &change, Metadata: change.Metadata})
	if err != nil {
		return nil, err
	}

	return &CardMessage{Message: *message}, nil
}

func (b *Bot) GetCardMessage(ctx context.Context, sessionID, messageID string) (*CardMessage, error) {
	message, err := b.GetMessage(ctx, sessionID, messageID)
	if err != nil {
		return nil, err
	}

	return &CardMessage{Message: *message}, nil
}
