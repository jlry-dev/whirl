package config

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

/*
Initilize the database and returns a connection pool on success, otherwise will stop the program.
*/
func InitDB() *pgxpool.Pool {
	config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_CONN_STR"))
	if err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("failed to ping pool: %v", err)
	}

	return pool
}
