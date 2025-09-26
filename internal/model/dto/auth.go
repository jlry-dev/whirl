package dto

type RegisterDTO struct {
	Username        string `json:"username" validate:"required,min=3,max=32,alphanum,excludesrune= "`
	Email           string `json:"email" validate:"required,email,max=255"`
	Password        string `json:"password" validate:"required,min=8,max=128"`
	ConfirmPassword string `json:"confirm-password" validate:"required,eqfield=Password"`
	Bio             string `json:"bio" validate:"max=255"`
	BirthDate       string `json:"birthdate" validate:"required,dateformat,age=13"`
	CountryCode     string `json:"country-code" validate:"required,iso3166_1_alpha3,min=3,max=3"`
}

type RegisterSuccessDTO struct {
	Status int            `json:"status"`
	Token  string         `json:"token"`
	User   map[string]any `json:"user"`
}

type LoginDTO struct {
	Username string `json:"username" validate:"required,min=3,max=32,alphanum,excludesrune= "`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

type LoginSuccessDTO struct {
	UserWithCountryDTO
	Status int    `json:"status"`
	Token  string `json:"token"`
}
