package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
)

var ErrDuplicateFriendship = errors.New("repo: friendship already exist")

type FriendshipRepo struct{}

func NewFriendshipRepository() FriendshipRepository {
	return &FriendshipRepo{}
}

func (f *FriendshipRepo) CreateFriendship(ctx context.Context, qr Queryer, fr *model.Friendship) error {
	qry := `INSERT INTO "friendship" (user1_id, user2_id) VALUES ($1, $2)`

	if _, err := qr.Exec(ctx, qry, fr.UID_1, fr.UID_2); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return ErrDuplicateFriendship
			}
		}

		return fmt.Errorf("repo: failed to create friendship : %w", err)
	}

	return nil
}

func (f *FriendshipRepo) DeleteFriendship(ctx context.Context, qr Queryer, fr *model.Friendship) error {
	qry := `DELETE FROM "friendship" as f WHERE (f.user1_id = $1 AND f.user2_id = $2) OR (f.user1_id = $2 AND f.user2_id = $1)`

	result, err := qr.Exec(ctx, qry, fr.UID_1, fr.UID_2)
	if err != nil {
		return fmt.Errorf("repo: failed to delete friendship : %w", err)
	}

	if result.RowsAffected() != 1 {
		return ErrNoRowsFound
	}

	return nil
}

func (f *FriendshipRepo) UpdateFriendshipStatus(ctx context.Context, qr Queryer, fr *model.Friendship) error {
	qry := `UPDATE "friendship" as f SET f.status = $1 WHERE (f.user1_id = $2 AND f.user2_id = $3) OR (f.user1_id = $3 AND f.user2_id = $2)`

	result, err := qr.Exec(ctx, qry, fr.UID_1, fr.UID_2)
	if err != nil {
		return fmt.Errorf("repo: failed to delete friendship : %w", err)
	}

	if result.RowsAffected() != 1 {
		return ErrNoRowsFound
	}

	return nil
}

func (f *FriendshipRepo) GetFriends(ctx context.Context, qr Queryer, userID, page int) ([]*dto.FriendDetails, error) {
	qry := `SELECT au.id, au.username, au.bio, au.bdate, a.public_url, c.name AS country_name, c.iso_code_3
		FROM app_user AS au
		LEFT JOIN avatar AS a ON au.avatar_id = a.id
		LEFT JOIN country AS c ON au.country_id = c.id
		WHERE au.id IN (
		    SELECT f.user1_id
		    FROM friendship f
		    WHERE f.user2_id = $1
		    UNION
		    SELECT f.user2_id
		    FROM friendship f
		    WHERE f.user1_id = $1
		LIMIT $2
		);`

	p := 10 * page
	rows, err := qr.Query(ctx, qry, userID, p)
	if err != nil {
		return nil, fmt.Errorf("repo: failed to get friends : %w", err)
	}
	defer rows.Close()

	friends := make([]*dto.FriendDetails, 0, 100)
	for rows.Next() {
		var u dto.FriendDetails
		err := rows.Scan(u.ID, u.Username, u.Bio, u.Bdate, u.Avatar, u.CountryName, u.CountryCode)
		if err != nil {
			return nil, fmt.Errorf("repo: failed to scan friend row : %w", err)
		}

		friends = append(friends, &u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: error during iteration : %w", err)
	}

	return friends, nil
}

func (f *FriendshipRepo) CheckRelationship(ctx context.Context, qr Queryer, fr *model.Friendship) (bool, error) {
	qry := `SELECT (id) FROM "friendship" as f WHERE (f.user1_id = $2 AND f.user2_id = $3) OR (f.user1_id = $3 AND f.user2_id = $2)`

	result, err := qr.Exec(ctx, qry, fr.UID_1, fr.UID_2)
	if err != nil {
		return false, fmt.Errorf("repo: failed to check friendship : %w", err)
	}

	if result.RowsAffected() != 1 {
		return false, nil
	}

	return true, nil
}
