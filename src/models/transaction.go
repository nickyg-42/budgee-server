package models

import "time"

type Transaction struct {
	ID          string    `json:"id"`
	AccountID   string    `json:"account_id"`
	Amount      string    `json:"amount"`
	Description string    `json:"description"`
	Date        string    `json:"date"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
}
