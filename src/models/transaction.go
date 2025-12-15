package models

import "time"

type Transaction struct {
	ID                             int       `json:"id"`
	AccountID                      int       `json:"account_id"`
	TransactionID                  string    `json:"transaction_id"`
	PrimaryCategory                *string   `json:"primary_category"`
	DetailedCategory               *string   `json:"detailed_category"`
	Type                           string    `json:"type"`
	Name                           string    `json:"name"`
	MerchantName                   *string   `json:"merchant_name"`
	Amount                         float64   `json:"amount"`
	Currency                       *string   `json:"currency"`
	Date                           time.Time `json:"date"`
	Pending                        bool      `json:"pending"`
	AccountOwner                   *string   `json:"account_owner"`
	CreatedAt                      time.Time `json:"created_at"`
	UpdatedAt                      time.Time `json:"updated_at"`
	PaymentChannel                 *string   `json:"payment_channel"`
	PersonalFinanceCategoryIconURL *string   `json:"personal_finance_category_icon_url"`
}
