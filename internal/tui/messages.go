package tui

type PhaseChangeMsg struct {
	TicketID  string
	Phase     Phase
	Status    string
	JSONLPath string
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
