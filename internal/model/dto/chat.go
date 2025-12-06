package dto

type ChatData struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}
