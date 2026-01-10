package models

type UpdateTransactionRequest struct {
	Amount                         float64
	PrimaryCategory                string
	DetailedCategory               string
	MerchantName                   string
	Date                           string
	PaymentChannel                 string
	PersonalFinanceCategoryIconURL string
}
