package textutil

import (
	"html"
	"regexp"
	"strings"
)

var (
	tagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

	plainURLPattern = regexp.MustCompile(
		`(?i)(https?://|www\.)[^\s<]+`,
	)
)

func ConvertLinksToAnchor(input string) string {
	var builder strings.Builder

	builder.Grow(len(input))

	last := 0

	for _, location := range tagPattern.FindAllStringIndex(input, -1) {
		builder.WriteString(linkifyText(input[last:location[0]]))
		builder.WriteString(input[location[0]:location[1]])

		last = location[1]
	}

	builder.WriteString(linkifyText(input[last:]))

	return builder.String()
}

func linkifyText(text string) string {
	return plainURLPattern.ReplaceAllStringFunc(text, func(raw string) string {
		href := raw

		if strings.HasPrefix(strings.ToLower(raw), "www.") {
			href = "https://" + raw
		}

		return `<a href="` + html.EscapeString(href) + `" rel="noopener noreferrer nofollow">` + html.EscapeString(raw) + `</a>`
	})
}
