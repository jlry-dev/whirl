package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
)

type UserRepository interface {
	CreateUser(ctx context.Context, qr Queryer, user *model.User) (id int, err error)
	UpdateAvatar(ctx context.Context, qr Queryer, user *model.User) (err error)
	GetUserWithCountryByUsername(ctx context.Context, qr Queryer, username string) (*dto.UserWithCountryDTO, error)
	CheckUsers(ctx context.Context, qr Queryer, userIDs ...int) (bool, error)
}

type AvatarRepository interface {
	CreateAvatar(ctx context.Context, qr Queryer, avatar *model.Avatar) (*model.Avatar, error)
	GetAvatarByPhash(ctx context.Context, qr Queryer, pHash string) (*model.Avatar, error)
}

type CountryRepository interface {
	GetIDByISO(ctx context.Context, qr Queryer, iso string) (id int, err error)
}

type FriendshipRepository interface {
	CreateFriendship(ctx context.Context, qr Queryer, fr *model.Friendship) error
	DeleteFriendship(ctx context.Context, qr Queryer, fr *model.Friendship) error
	UpdateFriendshipStatus(ctx context.Context, qr Queryer, fr *model.Friendship) error
	GetFriends(ctx context.Context, qr Queryer, userID, page int) ([]*dto.FriendDetails, error)
	CheckRelationship(ctx context.Context, qr Queryer, fr *model.Friendship) (bool, error)
}

type MessageRepository interface {
	CreateMessage(ctx context.Context, qr Queryer, ch *model.Message) error
	GetMessages(ctx context.Context, qr Queryer, uidOne, uidTwo, page int) ([]*model.Message, error)
}

type Queryer interface {
	Exec(ctx context.Context, query string, args ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
