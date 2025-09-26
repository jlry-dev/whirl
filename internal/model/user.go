package model

import "time"

type User struct {
	ID        int
	Username  string
	Email     string
	Password  string
	Bio       string
	Bdate     time.Time
	CreatedAt time.Time
	CountryID int
	Verified  bool
	AvatarID  int
}
