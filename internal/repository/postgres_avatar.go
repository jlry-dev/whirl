package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jlry-dev/whirl/internal/model"
)

var (
	ErrAvatarNotExist = errors.New("repo: avatar does not exist")
)

type AvatarRepo struct{}

func NewAvatarRepository() AvatarRepository {
	return &AvatarRepo{}
}

// CreateAvatar inserts the avatar and returns the stored row. If a concurrent
// request inserted the same p_hash first, the existing row is returned instead
// of an error, so duplicate uploads converge on a single avatar.
func (r *AvatarRepo) CreateAvatar(ctx context.Context, qr Queryer, avatar *model.Avatar) (*model.Avatar, error) {
	query := `INSERT INTO "avatar" (p_hash, public_id, asset_id, url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT ("p_hash") DO UPDATE SET "p_hash" = EXCLUDED."p_hash"
		RETURNING id, p_hash, public_id, asset_id, url`

	created := &model.Avatar{}

	if err := qr.QueryRow(ctx, query, avatar.PHash, avatar.PublicID, avatar.AssetID, avatar.URL).
		Scan(&created.ID, &created.PHash, &created.PublicID, &created.AssetID, &created.URL); err != nil {
		return &model.Avatar{}, fmt.Errorf("repo: failed to create avatar: %w", err)
	}

	return created, nil
}

func (r *AvatarRepo) GetAvatarByPhash(ctx context.Context, qr Queryer, pHash string) (*model.Avatar, error) {
	query := `SELECT id, p_hash, public_id, asset_id, url FROM "avatar" WHERE p_hash = $1`

	avatar := &model.Avatar{}

	if err := qr.QueryRow(ctx, query, pHash).Scan(&avatar.ID, &avatar.PHash, &avatar.PublicID, &avatar.AssetID, &avatar.URL); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.Avatar{}, ErrAvatarNotExist
		}

		return &model.Avatar{}, fmt.Errorf("repo: failed to get avatar id by phash : %w", err)
	}

	return avatar, nil
}
