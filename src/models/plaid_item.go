package models

type PlaidItem struct {
	ID          string `json:"id"`
	UserID      int64  `json:"user_id"`
	AccessToken string `json:"access_token"`
	ItemID      string `json:"item_id"`
	CreatedAt   string `json:"created_at"`
}
