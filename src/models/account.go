package models

import "time"

type Account struct {
	ID               string    `json:"id"`
	ItemID           string    `json:"item_id"`
	AccountID        string    `json:"account_id"`
	Name             string    `json:"name"`
	OfficialName     string    `json:"official_name"`
	Mask             string    `json:"mask"`
	Type             string    `json:"type"`
	Subtype          string    `json:"subtype"`
	CurrentBalance   string    `json:"current_balance"`
	AvailableBalance string    `json:"available_balance"`
	CreatedAt        time.Time `json:"created_at"`
}
