package dto

import "time"

type FriendshipDTO struct {
	From   int    `json:"from" validate:"required"`
	To     int    `json:"to" validate:"required"`
	Status string `json:"status" validate:"oneof=accepted blocked"`
}

type FrienshipServiceSuccessDTO struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type FriendDetails struct {
	ID          int       `json:"id"`
	Username    string    `json:"username"`
	Bio         string    `json:"bio"`
	Bdate       time.Time `json:"bdate"`
	CountryCode string    `json:"country-code"`
	CountryName string    `json:"country-name"`
	Avatar      string    `json:"avatar"`
}

type FriendsDetailsResponse struct {
	Status  int              `json:"json"`
	Friends []*FriendDetails `json:"friends"`
}
