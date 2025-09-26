package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jlry-dev/whirl/internal/model"
)

var (
	ErrDuplicateAvatar = errors.New("repo: duplicate avatar")
	ErrAvatarNotExist  = errors.New("repo: avatar does not exist")
)

type AvatarRepo struct{}

func NewAvatarRepository() AvatarRepository {
	return &AvatarRepo{}
}

/*
Uploads the avatar details to the database and returns the ID and URL

In cases where the avatar information already exist, then the existing record will be used.
*/
func (r *AvatarRepo) CreateAvatar(ctx context.Context, qr Queryer, avatar *model.Avatar) (*model.Avatar, error) {
	query := `INSERT INTO "avatar" (p_hash, public_id, asset_id, url) VALUES ($1, $2, $3, $4) ON CONFLICT RETURNING id, url`
	var aid int // Avatar ID
	var url string

	if err := qr.QueryRow(ctx, query, avatar.PHash, avatar.PublicID, avatar.AssetID, avatar.URL).Scan(&aid, &url); err != nil {
		return &model.Avatar{}, fmt.Errorf("repo: failed to create avatar: %w ", err)
	}

	return &model.Avatar{
		ID:  aid,
		URL: url,
	}, nil
}

func (r *AvatarRepo) GetAvatarByPhash(ctx context.Context, qr Queryer, pHash string) (*model.Avatar, error) {
	query := `SELECT (id, p_hash, public_id, asset_id, url) FROM "avatar" WHERE p_hash = $1`

	avatar := &model.Avatar{}

	if err := qr.QueryRow(ctx, query, pHash).Scan(&avatar.ID, &avatar.PHash, &avatar.PublicID, &avatar.AssetID, &avatar.URL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.Avatar{}, ErrAvatarNotExist
		}

		return &model.Avatar{}, fmt.Errorf("repo: failed to get avatar id by phash : %w", err)
	}

	return avatar, nil
}
