package dto

import (
	"mime/multipart"
	"time"
)

type UpdateAvatarDTO struct {
	ImgFile multipart.File
	UserID  int
}

type UpdateAvatarSuccessDTO struct {
	Status    int    `json:"status"`
	AvatarURL string `json:"avatar_url"`
}

type UserWithCountryDTO struct {
	ID          int       `json:"id,omitempty"`
	Username    string    `json:"username,omitempty"`
	Email       string    `json:"email,omitempty"`
	Password    string    `json:"-"`
	Bio         string    `json:"bio,omitempty"`
	Bdate       time.Time `json:"birthdate"`
	AvatarURL   string    `json:"avatar_url"`
	CountryCode string    `json:"country-code,omitempty"`
	CountryName string    `json:"country-name,omitempty"`
}
