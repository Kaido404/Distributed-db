package shared

type Request struct {
	Token     string `json:"token"`
	Query     string `json:"query"`
	Timestamp int64  `json:"timestamp,omitempty"`
	FromSlave string `json:"from_slave,omitempty"`
	IsSelect  bool   `json:"is_select,omitempty"`
}

type Response struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Rows    [][]any  `json:"rows,omitempty"`
	Header  []string `json:"header,omitempty"`
}
