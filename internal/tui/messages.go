package tui

type AskQuestionMsg struct {
	TicketID string
	Text     string
	Options  []string
	Response chan QuestionResponse
}

type QuestionResponse struct {
	OptionIndex int    // -1 for free text
	Text        string
}

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
