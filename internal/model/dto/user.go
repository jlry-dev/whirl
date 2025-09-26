package dto

import "mime/multipart"

type UpdateAvatarDTO struct {
	ImgFile multipart.File
	UserID  int
}

type UpdateAvatarSuccessDTO struct {
	Status    int    `json:"status"`
	AvatarURL string `json:"avatar_url"`
}
