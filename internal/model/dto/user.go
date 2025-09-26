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
	ID          int       `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Password    string    `json:"-"`
	Bio         string    `json:"bio"`
	Bdate       time.Time `json:"birthdate"`
	CountryCode string    `json:"country-code"`
	CountryName string    `json:"country-name"`
}
