package dto

type FriendshipDTO struct {
	From   int    `json:"from" validate:"required"`
	To     int    `json:"to" validate:"required"`
	Status string `json:"status" validate:"oneof=accepted blocked"`
}

type FrienshipServiceSuccessDTO struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}
