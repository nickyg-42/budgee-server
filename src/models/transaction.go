package models

import "time"

type Transaction struct {
	ID              int       `json:"id"`
	AccountID       int       `json:"account_id"`
	TransactionID   string    `json:"transaction_id"`
	PlaidCategoryID *string   `json:"plaid_category_id"`
	Category        *string   `json:"category"`
	Type            string    `json:"type"`
	Name            string    `json:"name"`
	MerchantName    *string   `json:"merchant_name"`
	Amount          float64   `json:"amount"`
	Currency        *string   `json:"currency"`
	Date            time.Time `json:"date"`
	Pending         bool      `json:"pending"`
	AccountOwner    *string   `json:"account_owner"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
