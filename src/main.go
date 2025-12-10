package main

import (
	"budgee-server/src/api"
	"budgee-server/src/config"
	sql "budgee-server/src/db"
	db "budgee-server/src/db/sql"
	"budgee-server/src/models"
	plaidclient "budgee-server/src/plaid"
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/plaid/plaid-go/plaid"
)

func main() {
	cfg := config.Load()

	logFile, err := os.OpenFile("/var/log/budgee-api.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: Failed to open log file, defaulting to stdout: %v", err)
	} else {
		defer logFile.Close()
		log.SetOutput(logFile)
		log.Println("Log file attached...")
	}

	// Connect to database
	pool, err := sql.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer pool.Close()

	// Initialize Plaid Client
	plaidClient := plaidclient.NewPlaidClient(cfg.PlaidClientID, cfg.PlaidSecret, cfg.PlaidEnvironment)

	// Router
	router := api.NewRouter(pool, plaidClient)

	// Start daily sync goroutine
	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, time.UTC)
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))
			RunDailySync(pool, plaidClient)
		}
	}()

	log.Println("API server running on port", cfg.Port)
	if err := http.ListenAndServe("127.0.0.1:"+cfg.Port, router); err != nil {
		log.Fatal(err)
	}
}

func RunDailySync(pool *pgxpool.Pool, plaidClient *plaid.APIClient) {
	ctx := context.Background()
	items, err := db.GetAllPlaidItems(ctx, pool)
	if err != nil {
		log.Printf("ERROR: Daily sync failed to get plaid items: %v", err)
		return
	}
	for _, item := range items {
		err := SyncTransactionsForItem(ctx, pool, plaidClient, item)
		if err != nil {
			log.Printf("ERROR: Daily sync failed for item %s (user %d): %v", item.ID, item.UserID, err)
		} else {
			log.Printf("INFO: Daily sync succeeded for item %s (user %d)", item.ID, item.UserID)
		}
	}
}

func SyncTransactionsForItem(ctx context.Context, pool *pgxpool.Pool, plaidClient *plaid.APIClient, item models.PlaidItem) error {
	itemIDInt, err := strconv.ParseInt(item.ID, 10, 64)
	if err != nil {
		return err
	}

	cursor, err := db.GetSyncCursor(ctx, pool, itemIDInt)
	if err != nil {
		return err
	}

	hasMore := true
	var allAdded []plaid.Transaction
	var allModified []plaid.Transaction
	var allRemoved []plaid.RemovedTransaction

	for hasMore {
		request := plaid.NewTransactionsSyncRequest(item.AccessToken)
		if cursor != "" {
			request.SetCursor(cursor)
		}
		transactionsResp, _, err := plaidClient.PlaidApi.TransactionsSync(ctx).TransactionsSyncRequest(*request).Execute()
		if err != nil {
			return err
		}
		allAdded = append(allAdded, transactionsResp.GetAdded()...)
		allModified = append(allModified, transactionsResp.GetModified()...)
		allRemoved = append(allRemoved, transactionsResp.GetRemoved()...)
		hasMore = transactionsResp.GetHasMore()
		cursor = transactionsResp.GetNextCursor()
	}

	// Save added transactions
	if err := db.SaveTransactions(ctx, pool, item.UserID, allAdded); err != nil {
		return err
	}
	// Update modified transactions
	if err := db.UpdateTransactions(ctx, pool, item.UserID, allModified); err != nil {
		return err
	}
	// Remove deleted transactions
	if err := db.RemoveTransactions(ctx, pool, item.UserID, allRemoved); err != nil {
		return err
	}
	// Update sync cursor
	if err := db.UpdateSyncCursor(ctx, pool, itemIDInt, cursor); err != nil {
		return err
	}
	return nil
}
