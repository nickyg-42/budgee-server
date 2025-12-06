package models

import "time"

type Transaction struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"account_id"`
	Amount       string    `json:"amount"`
	Description  string    `json:"description"`
	Date         time.Time `json:"date"`
	Category     string    `json:"category"`
	MerchantName *string   `json:"merchant_name"`
	Currency     *string   `json:"currency"`
	AccountOwner *string   `json:"account_owner"`
	CreatedAt    time.Time `json:"created_at"`
}
