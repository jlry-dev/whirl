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

/*
normalizePair returns the pair ordered as (low, high).

A friendship is an unordered pair, so both (a, b) and (b, a) must map to the
same stored row. The friendship table stores the pair in canonical (low, high)
order and enforces low < high with its friendship_ordered check constraint.
*/
func normalizePair(a, b int) (int, int) {
	if a > b {
		return b, a
	}

	return a, b
}

func (f *FriendshipRepo) CreateFriendship(ctx context.Context, qr Queryer, fr *model.Friendship) error {
	user1, user2 := normalizePair(fr.UID_1, fr.UID_2)

	qry := `INSERT INTO "friendship" (user1_id, user2_id) VALUES ($1, $2)`

	if _, err := qr.Exec(ctx, qry, user1, user2); err != nil {
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

	if result.RowsAffected() == 0 {
		return ErrNoRowsFound
	}

	return nil
}

func (f *FriendshipRepo) UpdateFriendshipStatus(ctx context.Context, qr Queryer, fr *model.Friendship) error {
	qry := `UPDATE friendship
		SET status = $1
		WHERE (user1_id = $2 AND user2_id = $3) OR (user1_id = $3 AND user2_id = $2)`

	result, err := qr.Exec(ctx, qry, fr.Status, fr.UID_1, fr.UID_2)
	if err != nil {
		return fmt.Errorf("repo: failed to update friendship : %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNoRowsFound
	}

	return nil
}

func (f *FriendshipRepo) GetFriends(ctx context.Context, qr Queryer, userID, page int) ([]*dto.FriendDetails, error) {
	qry := `SELECT au.id, au.username, au.bio, au.bdate, a.url, c.name AS country_name, c.iso_code_3
		FROM app_user AS au
		LEFT JOIN avatar AS a ON au.avatar_id = a.id
		LEFT JOIN country AS c ON au.country_id = c.id
		WHERE au.id IN (
		    SELECT f.user1_id
		    FROM friendship f
		    WHERE f.user2_id = $1 AND f.status <> 'blocked'
		    UNION
		    SELECT f.user2_id
		    FROM friendship f
		    WHERE f.user1_id = $1 AND f.status <> 'blocked'
		LIMIT $2
		)`

	p := 10 * page
	rows, err := qr.Query(ctx, qry, userID, p)
	if err != nil {
		return nil, fmt.Errorf("repo: failed to get friends : %w", err)
	}
	defer rows.Close()

	friends := make([]*dto.FriendDetails, 0, 100)
	for rows.Next() {
		var u dto.FriendDetails
		err := rows.Scan(&u.ID, &u.Username, &u.Bio, &u.Bdate, &u.Avatar, &u.CountryName, &u.CountryCode)
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
	qry := `SELECT EXISTS (
		SELECT 1 FROM "friendship" AS f
		WHERE (f.user1_id = $1 AND f.user2_id = $2) OR (f.user1_id = $2 AND f.user2_id = $1)
	)`

	var exists bool
	if err := qr.QueryRow(ctx, qry, fr.UID_1, fr.UID_2).Scan(&exists); err != nil {
		return false, fmt.Errorf("repo: failed to check friendship : %w", err)
	}

	return exists, nil
}
