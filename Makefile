# Include the .env file if it exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif


init-db:
	docker run -idt --name postgres16.10 -p 5432:5432 --mount src=whirl-db-volume,target=/var/lib/postgresql/data --env-file .env postgres:16.10-alpine

start-db:
	docker start postgres16.10

stop-db:
	docker stop postgres16.10

migrate-db-up:
	migrate -path=./db/migrations/ -database=$(DATABASE_CONN_STR) up

migrate-db-down:
	migrate -path=./db/migrations/ -database=$(DATABASE_CONN_STR) down

.PHONY: postgres migrate-db-up migrate-db-down
