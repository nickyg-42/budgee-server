1. run postgres: docker compose up -d
2. apply migrations: migrate -path src/db/migrations -database "postgres://postgres:password@localhost:5432/budgee?sslmode=disable" up
3. run the app: go run ./src/main.go

create a new migration: migrate create -ext sql -dir src/db/migrations -seq <migration name here>

docker compose down -v   # removes DB + volume
docker compose up -d     # recreate empty DB
