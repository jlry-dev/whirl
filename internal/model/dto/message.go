package dto

import "github.com/jlry-dev/whirl/internal/model"

type ChatData struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

type MessagesDTO struct {
	Status   int              `json:"status"`
	Messages []*model.Message `json:"messages"`
}
