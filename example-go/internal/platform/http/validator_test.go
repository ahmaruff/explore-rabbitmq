package http

import "testing"

type wordingRequest struct {
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required"`
	PasswordConfirm string `json:"password_confirmation" validate:"eqfield=Password"`
	Age             int    `json:"age" validate:"gte=18"`
}

type sizeAndFormatRequest struct {
	Title   string   `json:"title" validate:"min=3"`
	Tags    []string `json:"tags" validate:"min=2"`
	State   string   `json:"state" validate:"oneof=draft published"`
	Country string   `json:"country" validate:"iso3166_1_alpha2"`
	Website string   `json:"website" validate:"url"`
	Code    string   `json:"code" validate:"uuid4"`
}

// The map is keyed by json field name, not by Go field name, and a field that
// breaks a rule still reports a sentence the client can show.
func TestValidationMessagesUseJSONNames(t *testing.T) {
	req := wordingRequest{
		PasswordConfirm: "secret",
	}

	fields, ok := FormatValidationError(NewRequestValidator().Validate(&req))

	if !ok {
		t.Fatalf("expected validation to fail")
	}

	want := map[string]string{
		"email":                 "The email field is required.",
		"password":              "The password field is required.",
		"password_confirmation": "The password confirmation field must match the password field.",
		"age":                   "The age field must be greater than or equal to 18.",
	}

	if len(fields) != len(want) {
		t.Fatalf("got %d fields, want %d: %v", len(fields), len(want), fields)
	}

	for field, message := range want {
		if got := fields[field][0]; got != message {
			t.Errorf("%s message = %q, want %q", field, got, message)
		}
	}
}

// Sizes read as characters for a string and as items for a slice, so the same
// tag never sounds wrong.
func TestValidationMessagesMatchTheFieldKind(t *testing.T) {
	req := sizeAndFormatRequest{
		Title:   "ab",
		Tags:    []string{"one"},
		State:   "archived",
		Country: "IDN",
		Website: "not a url",
		Code:    "nope",
	}

	fields, ok := FormatValidationError(NewRequestValidator().Validate(&req))

	if !ok {
		t.Fatalf("expected validation to fail")
	}

	want := map[string]string{
		"title":   "The title field must be at least 3 characters.",
		"tags":    "The tags field must contain at least 2 items.",
		"state":   "The selected state is invalid.",
		"country": "The country field must be a valid two letter country code.",
		"website": "The website field must be a valid URL.",
		"code":    "The code field must be a valid UUID v4.",
	}

	if len(fields) != len(want) {
		t.Fatalf("got %d fields, want %d: %v", len(fields), len(want), fields)
	}

	for field, message := range want {
		if got := fields[field][0]; got != message {
			t.Errorf("%s message = %q, want %q", field, got, message)
		}
	}
}
