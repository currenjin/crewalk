package tui

type AskQuestionMsg struct {
	TicketID string
	Text     string
	Response chan string
}

type PhaseChangeMsg struct {
	TicketID string
	Phase    Phase
	Status   string
}

type StartTicketMsg struct {
	TicketID string
}

type AddTicketMsg struct {
	TicketID string
	Status   string
}

type TicketErrorMsg struct {
	TicketID string
	Err      error
}

type StatusMsg struct {
	Text string
}
