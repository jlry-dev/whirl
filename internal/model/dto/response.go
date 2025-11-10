package dto

type JSONResponse struct {
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type JSONError struct {
	Status int               `json:"status,omitempty"`
	Error  string            `json:"error,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}
