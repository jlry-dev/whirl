package service_test

import (
	"context"
	"os"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/jlry-dev/whirl/internal/service"
	"github.com/jlry-dev/whirl/internal/util"
	"github.com/jlry-dev/whirl/test/mocks"
)

func Test_Register(t *testing.T) {
	vld := validator.New(validator.WithRequiredStructEnabled())
	vld.RegisterValidation("age", util.ValidAgeValidator)
	vld.RegisterValidation("dateformat", util.DateFormatValidator)

	// Setup env variables
	os.Setenv("JWT_KEY", "testkey")

	testCase := []struct {
		name      string
		inp       *dto.RegisterDTO
		mockSetup func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo)
		expErr    error
		wantErr   bool
	}{
		{
			name: "valid registration with bio",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				Bio:             "This is about me and yeah hello there!",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {
				c.On("GetIDByISO", mock.Anything, mock.Anything, mock.Anything).Return(1, nil)
				u.On("CreateUser", mock.Anything, mock.Anything, mock.MatchedBy(func(u *model.User) bool {
					return u.Username == "john" && u.Email == "john@example.com" && u.CountryID == 1
				})).Return(10, nil)
			},
			wantErr: false,
		},
		{
			name: "valid registration without bio",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {
				c.On("GetIDByISO", mock.Anything, mock.Anything, "CAN").Return(1, nil)
				u.On("CreateUser", mock.Anything, mock.Anything, mock.MatchedBy(func(u *model.User) bool {
					return u.Username == "john" && u.Email == "john@example.com" && u.CountryID == 1
				})).Return(10, nil)
			},
			wantErr: false,
		},
		{
			name: "valid registration (username contains numbers)",
			inp: &dto.RegisterDTO{
				Username:        "john123",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {
				c.On("GetIDByISO", mock.Anything, mock.Anything, "CAN").Return(1, nil)
				u.On("CreateUser", mock.Anything, mock.Anything, mock.MatchedBy(func(u *model.User) bool {
					return u.Username == "john123" && u.Email == "john@example.com" && u.CountryID == 1
				})).Return(10, nil)
			},
			wantErr: false,
		},
		{
			name: "invalid dto (missing username)",
			inp: &dto.RegisterDTO{
				Username:        "",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid dto (missing email)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid dto (missing password)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid dto (missing confirm password)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid dto (missing birth date)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid dto (missing country code)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid email",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid email (contains spaces)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "jo hn@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid password (mismatch password)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "mismatchpassword",
				ConfirmPassword: "differentpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid username (too short)",
			inp: &dto.RegisterDTO{
				Username:        "jo",
				Email:           "jo@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid username (too long)",
			inp: &dto.RegisterDTO{
				Username:        "johnwithasuperlongahhnamefromsomewhere",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid username (contains space)",
			inp: &dto.RegisterDTO{
				Username:        "jo hn",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid username (contains special characters)",
			inp: &dto.RegisterDTO{
				Username:        "john$!-",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid bio (exceed max length) ",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				Bio:             "a super long character that exceeds the maximum characters needed, well idk how much longer I need this to be but yeah. I will just copy and paste it. a super long character that exceeds the maximum characters needed, well idk how much longer I need this to be but yeah. I will just copy and paste it",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid birthdate format (MM-YYYY-DD)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "01-2004-30",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid birthdate format (YYYY-DD-MM)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-20-05",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid birthdate format (MM-DD-YYYY) ",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "01-20-2004",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid birthdate (month exceeds 13) ",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-13-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid birthdate (day exceeds 31)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-33",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid birthdate (feb 30 on leap year)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-02-30",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid age (too young)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2023-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid age (future date)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2320-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid iso code (more than 3 letters)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CANA",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid iso code (lowercase letters)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "can",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {},
			expErr:    service.ErrValidationFailed,
			wantErr:   true,
		},
		{
			name: "invalid iso code (country not supported)",
			inp: &dto.RegisterDTO{
				Username:        "john",
				Email:           "john@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "USA",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {
				c.On("GetIDByISO", mock.Anything, mock.Anything, "USA").Return(0, repository.ErrCountryNotExist)
			},
			expErr:  service.ErrCountryNotSupported,
			wantErr: true,
		},
		{
			name: "invalid registration (duplicated user)",
			inp: &dto.RegisterDTO{
				Username:        "alreadyexist",
				Email:           "alreadyexist@example.com",
				Password:        "validpassword",
				ConfirmPassword: "validpassword",
				BirthDate:       "2004-05-20",
				CountryCode:     "CAN",
			},
			mockSetup: func(u *mocks.MockUserRepo, c *mocks.MockCountryRepo) {
				c.On("GetIDByISO", mock.Anything, mock.Anything, "CAN").Return(1, nil)
				u.On("CreateUser", mock.Anything, mock.Anything, mock.MatchedBy(func(u *model.User) bool {
					return u.Username == "alreadyexist" && u.Email == "alreadyexist@example.com" && u.CountryID == 1
				})).Return(0, repository.ErrDuplicateUser)
			},
			expErr:  service.ErrUserAlreadyExist,
			wantErr: true,
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			// Mocks
			userRepo := new(mocks.MockUserRepo)
			countryRepo := new(mocks.MockCountryRepo)

			tc.mockSetup(userRepo, countryRepo)

			// create a new service
			srv := service.NewAuthService(vld, userRepo, countryRepo, nil)
			resp, err := srv.Register(context.Background(), tc.inp)

			if tc.wantErr {
				t.Log(err.Error())
				assert.ErrorIs(t, err, tc.expErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tc.inp.Username, resp.User.Username)
				assert.Equal(t, tc.inp.Email, resp.User.Email)
				assert.NotEmpty(t, resp.Token)
			}

			userRepo.AssertExpectations(t)
			countryRepo.AssertExpectations(t)
		})
	}
}
