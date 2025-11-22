package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/plaid/plaid-go/plaid"
)

func CreateLinkToken(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get userID from params or jwt?
		user := plaid.LinkTokenCreateRequestUser{
			ClientUserId: strconv.FormatInt(userID, 10),
		}
		request := plaid.NewLinkTokenCreateRequest(
			"Budgee",
			"en",
			[]plaid.CountryCode{plaid.COUNTRYCODE_US},
			user,
		)
		request.SetProducts([]plaid.Products{plaid.PRODUCTS_TRANSACTIONS})
		resp, _, err := plaidClient.PlaidApi.LinkTokenCreate(context.Background()).LinkTokenCreateRequest(*request).Execute()
		if err != nil {
			http.Error(w, "Failed to create link token", http.StatusInternalServerError)
			log.Printf("ERROR: Plaid link token creation failed for user %d: %v", userID, err)
			return
		}

		linkToken := resp.GetLinkToken()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(linkToken)
	}
}

func ExchangePublicToken(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// get public token from request body
		exchangePublicTokenReq := plaid.NewItemPublicTokenExchangeRequest(publicToken)
		exchangePublicTokenResp, _, err := plaidClient.PlaidApi.ItemPublicTokenExchange(context.Background()).ItemPublicTokenExchangeRequest(
			*exchangePublicTokenReq,
		).Execute()

		if err != nil {
			http.Error(w, "Failed to exchange public token", http.StatusInternalServerError)
			log.Printf("ERROR: Plaid public token exchange failed: %v", err)
			return
		}

		accessToken := exchangePublicTokenResp.GetAccessToken()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(accessToken)
	}
}

func GetTransactions(plaidClient *plaid.APIClient, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Grab access token from db - do not pass from client

		request := plaid.NewTransactionsSyncRequest(accessToken)

		transactionsResp, _, err := plaidClient.PlaidApi.TransactionsSync(context.Background()).TransactionsSyncRequest(*request).Execute()
	}
}
