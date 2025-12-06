package model

import "time"

type Friendship struct {
	UID_1     int
	UID_2     int
	Status    FriendshipStatus
	CreatedAt time.Time
}

type FriendshipStatus string
