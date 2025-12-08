package model

import "time"

type Message struct {
	ID         int
	SenderID   int
	ReceiverID int
	Content    string
	Timestamp  time.Time
}
