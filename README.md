1. run postgres: docker compose up -d
2. apply migrations: migrate -path src/db/migrations -database "postgres://postgres:password@localhost:5432/budgee?sslmode=disable" up
3. run the app: go run ./src/main.go

create a new migration: migrate create -ext sql -dir src/db/migrations -seq <migration name here>

docker compose down -v   # removes DB + volume
docker compose up -d     # recreate empty DB

TODO:
1. DO NOT PASS ACCESS TOKEN TO FRONTEND EVER
2. make sure accounts show after linking institution https://plaid.com/docs/api/accounts/
3. Make transaction sync work