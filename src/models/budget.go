package models

import "time"

type Budget struct {
	ID                      int       `json:"id"`
	UserID                  int       `json:"user_id"`
	Amount                  float64   `json:"amount"`
	PersonalFinanceCategory string    `json:"personal_finance_category"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}
