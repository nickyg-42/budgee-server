package models

import "time"

type PlaidItem struct {
	ID              string    `json:"id"`
	UserID          int64     `json:"user_id"`
	AccessToken     string    `json:"-"`
	ItemID          string    `json:"item_id"`
	InstitutionID   string    `json:"institution_id"`
	InstitutionName string    `json:"institution_name"`
	CreatedAt       time.Time `json:"created_at"`
}
