package proto

// Message is deliberately compact; payload is validated by the server.
type Message struct {
	Type     string         `json:"t"`
	Token    string         `json:"token,omitempty"`
	Name     string         `json:"name,omitempty"`
	Color    string         `json:"color,omitempty"`
	Hair     string         `json:"hair,omitempty"`
	Zone     string         `json:"zone,omitempty"`
	Act      string         `json:"act,omitempty"`
	Payload  map[string]any `json:"p,omitempty"`
	Channel  string         `json:"ch,omitempty"`
	Text     string         `json:"text,omitempty"`
	Target   string         `json:"target,omitempty"`
	Amount   int            `json:"amount,omitempty"`
	Price    int            `json:"price,omitempty"`
	ID       string         `json:"id,omitempty"`
	Choice   string         `json:"choice,omitempty"`
	Item     string         `json:"item,omitempty"`
	Score    int            `json:"score,omitempty"`
	Position int            `json:"pos,omitempty"`
	SentAt   int64          `json:"sentAt,omitempty"`
}
