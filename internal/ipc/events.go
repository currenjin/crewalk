package ipc

type EventType string

const (
	EventPhaseChange EventType = "phase_change"
	EventAskQuestion EventType = "ask_question"
	EventAnswer      EventType = "answer"
)

type Event struct {
	Type     EventType `json:"type"`
	TicketID string    `json:"ticket_id"`
	Phase    string    `json:"phase,omitempty"`
	Status   string    `json:"status,omitempty"`
	Text     string    `json:"text,omitempty"`
}
