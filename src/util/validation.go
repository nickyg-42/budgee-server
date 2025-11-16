package util

import (
	"regexp"
)

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
