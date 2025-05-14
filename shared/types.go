package shared

type DBRequest struct {
	Query     string `json:"query"`
	Token     string `json:"token"`
	FromSlave string `json:"from_slave"`
	IsSelect  bool   `json:"is_select"`
	IP        string `json:"ip"`
	Role      string `json:"role"`
}

type DBResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Role    string          `json:"role,omitempty"`
	Header  []string        `json:"header,omitempty"`
	Rows    [][]interface{} `json:"rows,omitempty"`
}

type TableColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
}

type CreateTableRequest struct {
	DBName    string        `json:"db_name"`
	TableName string        `json:"table_name"`
	Columns   []TableColumn `json:"columns"`
}

type ReplicationRequest struct {
	DBName    string `json:"db_name"`
	TableName string `json:"table_name"`
	Operation string `json:"operation"` 
	Data      []byte `json:"data"`      
}

type ReplicationResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
