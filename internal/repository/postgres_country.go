package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrCountryNotExist = errors.New("repo: country does not exist")

type CountryRepo struct{}

func NewCountryRepository() CountryRepository {
	return &CountryRepo{}
}

func (r *CountryRepo) GetIDByISO(ctx context.Context, qr Queryer, iso_code string) (int, error) {
	query := `SELECT id FROM "country" WHERE iso_code_3 = $1`

	var cid int // Country ID
	if err := qr.QueryRow(ctx, query, iso_code).Scan(&cid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrCountryNotExist
		}

		return 0, fmt.Errorf("repo: faield to get country id by iso: %w", err)
	}

	return cid, nil
}
