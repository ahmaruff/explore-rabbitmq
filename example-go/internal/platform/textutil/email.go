package textutil

import (
	"net/mail"
	"regexp"
	"strings"
)

var emailPattern = regexp.MustCompile(
	`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
)

func MaskEmail(email string) string {
	addr, err := mail.ParseAddress(email)

	if err != nil || addr.Address != email {
		return email
	}

	parts := strings.SplitN(email, "@", 2)

	username := parts[0]
	domain := parts[1]

	usernameRunes := []rune(username)
	maskedUsername := string(usernameRunes[:1]) +
		strings.Repeat("*", max(1, len(usernameRunes)-1))

	domainParts := strings.Split(domain, ".")

	if len(domainParts) < 2 {
		return email
	}

	maskedDomain := strings.Repeat("*", len([]rune(domainParts[0]))) +
		"." + domainParts[len(domainParts)-1]

	return maskedUsername + "@" + maskedDomain
}

func MaskEmailsInText(input string) string {
	if !strings.Contains(input, "@") {
		return input
	}

	return emailPattern.ReplaceAllStringFunc(input, func(email string) string {
		return MaskEmail(email)
	})
}
