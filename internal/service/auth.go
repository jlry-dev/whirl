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

var (
	ErrValidationFailed    = errors.New("service: failed to validate data")
	ErrUserAlreadyExist    = errors.New("service: user already exist")
	ErrCountryNotSupported = errors.New("service: country not supported / not exist")
)

type AuthService interface {
	Register(ctx context.Context, data *dto.RegisterDTO) (*dto.RegisterSuccessDTO, error)
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
		return nil, ErrValidationFailed
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
		return nil, fmt.Errorf("user reg: failed to hash password: %w", err)
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

		return nil, fmt.Errorf("auth service : failed to create user : %w", err)
	}

	token, err := util.GenerateJWT(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("auth service : failed to generate token : %w", err)
	}

	return &dto.RegisterSuccessDTO{
		Token: token,
		User: map[string]any{
			"username": data.Username,
			"email":    data.Email,
			"bio":      data.Bio,
			"country":  data.CountryCode,
		},
	}, nil
}
