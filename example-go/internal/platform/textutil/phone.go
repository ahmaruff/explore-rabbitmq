package textutil

import (
	"regexp"
	"strings"
)

var (
	phonePattern              = regexp.MustCompile(`^(?:\+62|62|08)\d{8,13}$`)
	internationalPhonePattern = regexp.MustCompile(`^\+\d{9,15}$`)

	numberPattern = regexp.MustCompile(`(?:\+?\d{1,3}|0)\d{8,13}`)

	phoneMaskPattern         = regexp.MustCompile(`^(\+62|62|08)(\d{3})\d+(\d{3})$`)
	internationalMaskPattern = regexp.MustCompile(`^(\+\d{1,3})(\d{3})\d+(\d{3})$`)
)

func MaskPhoneNumber(number string) string {
	number = strings.TrimSpace(number)

	if !IsPhoneNumber(number) {
		return number
	}

	if phonePattern.MatchString(number) {
		return maskPhone(number, phoneMaskPattern)
	}

	if strings.HasPrefix(number, "+") {
		return maskPhone(number, internationalMaskPattern)
	}

	return number
}

func MaskPhoneNumbersInText(input string) string {
	return numberPattern.ReplaceAllStringFunc(input, func(number string) string {
		return MaskPhoneNumber(number)
	})
}

func IsPhoneNumber(number string) bool {
	return phonePattern.MatchString(number) ||
		internationalPhonePattern.MatchString(number)
}

func maskPhone(number string, pattern *regexp.Regexp) string {
	matches := pattern.FindStringSubmatch(number)
	if len(matches) != 4 {
		return number
	}

	return matches[1] + matches[2] + "****" + matches[3]
}
