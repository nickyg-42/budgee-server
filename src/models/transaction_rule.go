package models

import (
	"encoding/json"
	"time"
)

type TransactionRule struct {
	ID                      int             `json:"id"`
	UserID                  int             `json:"user_id"`
	Name                    string          `json:"name"`
	Conditions              json.RawMessage `json:"conditions"` // JSONB
	PersonalFinanceCategory string          `json:"personal_finance_category"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}
