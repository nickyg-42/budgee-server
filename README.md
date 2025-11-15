1. docker compose up -d
2. migrate -path src/db/migrations "postgres://postgres:password@localhost:5432/budgee?sslmode=disable" -database up
3. go run ./src/main.go

docker compose down -v   # removes DB + volume
docker compose up -d     # recreate empty DB