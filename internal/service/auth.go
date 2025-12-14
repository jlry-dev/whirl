package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/jlry-dev/whirl/internal/util"
	"golang.org/x/crypto/bcrypt"
)

type ErrVldFailed struct {
	Fields map[string]string
}

func (e *ErrVldFailed) Error() string {
	return "failed to validate fields."
}

var (
	ErrUserAlreadyExist    = errors.New("service: user already exist")
	ErrCountryNotSupported = errors.New("service: country not supported / not exist")
	ErrNoUserExist         = errors.New("service: no user with credentials exist")
	ErrInvalidCredential   = errors.New("service: invalid / mismatch login credentials")
)

type AuthService interface {
	Register(ctx context.Context, data *dto.RegisterDTO) (*dto.RegisterSuccessDTO, error)
	Login(ctx context.Context, data *dto.LoginDTO) (*dto.LoginSuccessDTO, error)
}

type AuthSrv struct {
	validate    *validator.Validate
	userRepo    repository.UserRepository
	countryRepo repository.CountryRepository
	db          *pgxpool.Pool
}

func NewAuthService(validate *validator.Validate, userRepo repository.UserRepository, countryRepo repository.CountryRepository, db *pgxpool.Pool) AuthService {
	return &AuthSrv{
		validate:    validate,
		userRepo:    userRepo,
		countryRepo: countryRepo,
		db:          db,
	}
}

func (srv *AuthSrv) Register(ctx context.Context, data *dto.RegisterDTO) (*dto.RegisterSuccessDTO, error) {
	if err := srv.validate.Struct(data); err != nil {
		vldErrs := err.(validator.ValidationErrors)
		ve := ErrVldFailed{
			Fields: make(map[string]string),
		} // the error struct the holds a map of the field name to the validation message

		for _, e := range vldErrs {
			ve.Fields[e.Field()] = util.GetValidationMessage(e)
		}

		return nil, &ve
	}

	cid, err := srv.countryRepo.GetIDByISO(ctx, srv.db, data.CountryCode)
	if err != nil {
		if errors.Is(err, repository.ErrCountryNotExist) {
			return nil, ErrCountryNotSupported
		}

		return nil, err
	}

	// Parse the Bdate from the dto to convert it into time.Time
	pBdate, err := time.Parse(time.DateOnly, data.BirthDate)
	if err != nil {
		return nil, fmt.Errorf("reg service: failed to parse bdate DTO as time.Time : %w", err)
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(data.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("reg service: failed to hash password: %w", err)
	}

	user := &model.User{
		Username:  data.Username,
		Email:     data.Email,
		Password:  string(hashedPass),
		Bio:       data.Bio,
		Bdate:     pBdate,
		CountryID: cid,
	}

	uid, err := srv.userRepo.CreateUser(
		ctx,
		srv.db,
		user,
	)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateUser) {
			return nil, ErrUserAlreadyExist
		}

		return nil, fmt.Errorf("reg service : failed to create user : %w", err)
	}

	token, err := util.GenerateJWT(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("reg service : failed to generate token : %w", err)
	}

	userWithCountry := dto.UserWithCountryDTO{
		ID:          uid,
		Username:    data.Username,
		Email:       data.Email,
		Bio:         &data.Bio,
		Bdate:       pBdate,
		CountryCode: data.CountryCode,
	}

	return &dto.RegisterSuccessDTO{
		Token: token,
		User:  userWithCountry,
	}, nil
}

func (srv *AuthSrv) Login(ctx context.Context, data *dto.LoginDTO) (*dto.LoginSuccessDTO, error) {
	if err := srv.validate.Struct(data); err != nil {
		vldErrs := err.(validator.ValidationErrors)
		ve := ErrVldFailed{
			Fields: make(map[string]string),
		} // the error struct the holds a map of the field name to the validation message
		for _, e := range vldErrs {
			ve.Fields[e.Tag()] = util.GetValidationMessage(e)
		}

		return nil, &ve
	}

	userInfo, err := srv.userRepo.GetUserWithCountryByUsername(ctx, srv.db, data.Username)
	if err != nil {
		if errors.Is(err, repository.ErrNoRowsFound) {
			return nil, ErrNoUserExist
		}

		return nil, fmt.Errorf("login service: failed to get user : %w", err)
	}

	// Match the password
	if err := bcrypt.CompareHashAndPassword([]byte(userInfo.Password), []byte(data.Password)); err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return nil, ErrInvalidCredential
		}

		return nil, fmt.Errorf("login service: failed trying to match password : %w", err)
	}

	token, err := util.GenerateJWT(ctx, userInfo.ID)
	if err != nil {
		return nil, fmt.Errorf("login service: failed to generate jwt token : %w", err)
	}

	return &dto.LoginSuccessDTO{
		User:  userInfo,
		Token: token,
	}, nil
}
