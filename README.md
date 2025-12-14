1. run postgres: docker compose up -d
2. apply migrations: migrate -path src/db/migrations -database "postgres://postgres:password@localhost:5432/budgee?sslmode=disable" up
3. run the app: go run ./src/main.go

create a new migration: migrate create -ext sql -dir src/db/migrations -seq <migration name here>

docker compose down -v   # removes DB + volume
docker compose up -d     # recreate empty DB


# URGENT
 Add filter for income vs expense
 Find out why categories are all "Other"
 add savings/cash flow label on bar graph 
 SORT TRANSACTIONS BY DATE

# LESS IMPORTANT
make expenses show upside down on the graph
Better and deeper insights, possibly AI powered.
show year and/or day metrics
Some transactions are categorized properly, some arent?!
![alt text](image-1.png)

# TODO : rely on github secrets instead of .env?