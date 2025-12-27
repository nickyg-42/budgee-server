package util

import (
	"regexp"
	"strings"
)

// IsExpense determines if a transaction is a true expense, excluding transfers and credit card payments.
func IsExpense(accountType string, amount float64, categoryPrimary string) bool {
	// Exclude these primary categories
	exclude := map[string]struct{}{
		"TRANSFER":             {},
		"LOAN_PAYMENTS":        {},
		"CREDIT_CARD_PAYMENTS": {},
		"TRANSFER_OUT":         {},
	}
	// Only treat positive amounts as expenses (outflow)
	if amount <= 0 {
		return false
	}
	// Normalize category
	cat := strings.ToUpper(categoryPrimary)
	if _, found := exclude[cat]; found {
		return false
	}
	// Credit account: only purchases (not payments/credits)
	if accountType == "credit" {
		return true
	}
	// Depository: exclude transfers/payments
	if accountType == "depository" {
		return true
	}
	// Other account types: not counted as expenses
	return false
}

// IsIncome determines if a transaction is true income, excluding transfers and similar inflows.
func IsIncome(accountType string, amount float64, categoryPrimary string) bool {
	// Exclude these primary categories
	exclude := map[string]struct{}{
		"TRANSFER":             {},
		"LOAN_PAYMENTS":        {},
		"CREDIT_CARD_PAYMENTS": {},
		"TRANSFER_IN":          {},
	}
	// Only treat negative amounts as income (inflow)
	if amount >= 0 {
		return false
	}
	// Normalize category
	cat := strings.ToUpper(categoryPrimary)
	if _, found := exclude[cat]; found {
		return false
	}
	// Credit account: only refunds/credits (not payments)
	if accountType == "credit" {
		return true
	}
	// Depository: exclude transfers/payments
	if accountType == "depository" {
		return true
	}
	// Other account types: not counted as income
	return false
}

func ValidateEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

func ValidateUsername(username string) bool {
	return len(username) >= 3 && len(username) <= 30
}

func ValidatePassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	hasLower := regexp.MustCompile("[a-z]").MatchString(password)
	hasUpper := regexp.MustCompile("[A-Z]").MatchString(password)
	hasDigit := regexp.MustCompile("[0-9]").MatchString(password)
	hasSpecial := regexp.MustCompile(`[^A-Za-z0-9]`).MatchString(password)

	return hasLower && hasUpper && hasDigit && hasSpecial
}
