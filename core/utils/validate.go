package utils

import (
	"errors"
	"regexp"
)

var (
	usernameRe        = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,32}$`)
	passwordMaxLength = 128
	passwordMinLength = 12
	upperRe           = regexp.MustCompile(`[A-Z]`)
	lowerRe           = regexp.MustCompile(`[a-z]`)
	digitRe           = regexp.MustCompile(`[0-9]`)
	specialRe         = regexp.MustCompile(`[!@#$%^&*_\-+=]`)
	whitespaceRe      = regexp.MustCompile(`\s`)
)

func ValidateUsername(s string) error {
	if !usernameRe.MatchString(s) {
		return errors.New("invalid username")
	}
	return nil
}

func ValidatePassword(s string) error {
	if len(s) < passwordMinLength {
		return errors.New("password too short (min 12 chars)")
	}
	if len(s) > passwordMaxLength {
		return errors.New("password too long (max 128 chars)")
	}
	if whitespaceRe.MatchString(s) {
		return errors.New("password must not contain spaces")
	}
	if !upperRe.MatchString(s) {
		return errors.New("password must include at least one uppercase letter")
	}
	if !lowerRe.MatchString(s) {
		return errors.New("password must include at least one lowercase letter")
	}
	if !digitRe.MatchString(s) {
		return errors.New("password must include at least one digit")
	}
	if !specialRe.MatchString(s) {
		return errors.New("password must include at least one special character (!@#$%^&*_-+=)")
	}
	return nil
}
