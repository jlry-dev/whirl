package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
)

var (
	ErrDuplicateUser = errors.New("repo: duplicate user")
	ErrNoRowsFound   = errors.New("repo: no user exist")
)

type UserRepo struct{}

func NewUserRepository() UserRepository {
	return &UserRepo{}
}

// Returns the userID and an error
func (r *UserRepo) CreateUser(ctx context.Context, qr Queryer, user *model.User) (int, error) {
	// By default verified is set to false
	// By default created_at is set to the now()
	isrtQuery := `INSERT INTO app_user (username, email, password, bio, bdate, created_at, country_id, verified) VALUES ($1, $2, $3, $4, $5, default, $6, default) RETURNING id`

	var uid int // userID

	// Expecting a return of id, refer to isrtQuery
	if err := qr.QueryRow(ctx, isrtQuery, user.Username, user.Email, user.Password, user.Bio, user.Bdate, user.CountryID).Scan(&uid); err != nil {

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return 0, ErrDuplicateUser
			}
		}

		return 0, fmt.Errorf("repo: failed to create user: %w", err)
	}

	return uid, nil
}

func (r *UserRepo) UpdateAvatar(ctx context.Context, qr Queryer, user *model.User) error {
	qry := `UPDATE "app_user" SET avatar_id = $1 WHERE id = $2`

	_, err := qr.Exec(ctx, qry, user.AvatarID, user.ID)
	if err != nil {
		return fmt.Errorf("repo: failed to set avatar url : %w", err)
	}

	return nil
}

func (r *UserRepo) GetUserWithCountryByUsername(ctx context.Context, qr Queryer, username string) (*dto.UserWithCountryDTO, error) {
	qry := `SELECT u.id, u.username, u.email, u.password, u.bio, u.bdate, c.iso_code_3, c.name FROM "app_user" AS u JOIN "country" AS c ON u.country_id = c.id WHERE u.username = $1`

	userInfo := new(dto.UserWithCountryDTO)
	if err := qr.QueryRow(ctx, qry, username).Scan(&userInfo.ID, &userInfo.Username, &userInfo.Email, &userInfo.Password, &userInfo.Bio, &userInfo.Bdate, &userInfo.CountryCode, &userInfo.CountryName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoRowsFound
		}

		return nil, fmt.Errorf("repo: failed to get user: %w", err)
	}

	return userInfo, nil
}
