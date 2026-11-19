package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// dbConnectTimeout bounds the startup ping so a wrong host does not hang boot
// forever.
const dbConnectTimeout = 5 * time.Second

/*
Initilize the database and return a connection pool.

Kadtong log.Fatal dati kay gibalhin diri as error, para makita sa main ug
makahatag ug klaro nga mensahe (ug testable ang config).
*/
func InitDB() (*pgxpool.Pool, error) {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_CONN_STR"))
	if dsn == "" {
		return nil, errors.New("config: DATABASE_CONN_STR is required")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("config: invalid DATABASE_CONN_STR: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("config: failed to create db pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dbConnectTimeout)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("config: failed to connect to database: %w", err)
	}

	return pool, nil
}
