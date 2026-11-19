package model

import "time"

type Friendship struct {
	UID_1     int
	UID_2     int
	Status    FriendshipStatus
	BlockedBy *int
	CreatedAt time.Time
}

type FriendshipStatus string

const (
	FriendshipAccepted FriendshipStatus = "accepted"
	FriendshipBlocked  FriendshipStatus = "blocked"
)
