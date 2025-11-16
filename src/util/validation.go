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
	return len(password) >= 8 // enforce minimum 8 chars; can add more rules
}
