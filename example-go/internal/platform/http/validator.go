package http

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

type RequestValidator struct {
	validator *validator.Validate
}

func NewRequestValidator() *RequestValidator {

	v := validator.New(validator.WithRequiredStructEnabled())

	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]

		if name == "-" {
			return ""
		}

		if name == "" {
			return field.Name
		}

		return name

	})

	v.RegisterValidation("notblank", func(field validator.FieldLevel) bool {
		return strings.TrimSpace(field.Field().String()) != ""
	})

	return &RequestValidator{
		validator: v,
	}
}

func (v *RequestValidator) Validate(i any) error {
	return v.validator.Struct(i)
}

func FormatValidationError(err error) (map[string][]string, bool) {
	var validationErrors validator.ValidationErrors

	if !errors.As(err, &validationErrors) {
		return nil, false
	}

	fields := make(map[string][]string)

	for _, fieldErr := range validationErrors {
		field := fieldErr.Field()
		message := validationMessage(fieldErr)

		fields[field] = append(fields[field], message)
	}

	return fields, true

}

func validationMessage(fe validator.FieldError) string {
	field := strings.ReplaceAll(fe.Field(), "_", " ")
	param := fe.Param()

	switch fe.Tag() {

	// Required and other presence rules.
	case "required":
		return fmt.Sprintf("The %s field is required.", field)

	case "required_if":
		other, value := fieldParam(param)

		return fmt.Sprintf("The %s field is required when %s is %s.", field, other, value)

	case "required_unless":
		other, value := fieldParam(param)

		return fmt.Sprintf("The %s field is required unless %s is %s.", field, other, value)

	case "required_with":
		return fmt.Sprintf("The %s field is required when %s is present.", field, fieldLabel(param))

	case "required_with_all":
		return fmt.Sprintf("The %s field is required when %s are present.", field, fieldLabel(param))

	case "required_without":
		return fmt.Sprintf("The %s field is required when %s is not present.", field, fieldLabel(param))

	case "required_without_all":
		return fmt.Sprintf("The %s field is required when none of %s are present.", field, fieldLabel(param))

	case "excluded_if":
		other, value := fieldParam(param)

		return fmt.Sprintf("The %s field is not allowed when %s is %s.", field, other, value)

	case "excluded_unless":
		other, value := fieldParam(param)

		return fmt.Sprintf("The %s field is not allowed unless %s is %s.", field, other, value)

	case "excluded_with":
		return fmt.Sprintf("The %s field is not allowed when %s is present.", field, fieldLabel(param))

	case "excluded_with_all":
		return fmt.Sprintf("The %s field is not allowed when %s are present.", field, fieldLabel(param))

	case "excluded_without":
		return fmt.Sprintf("The %s field is not allowed when %s is not present.", field, fieldLabel(param))

	case "excluded_without_all":
		return fmt.Sprintf("The %s field is not allowed when none of %s are present.", field, fieldLabel(param))

	case "notblank":
		return fmt.Sprintf("The %s field must not be blank.", field)

	// Comparisons against a literal value.
	case "eq":
		return fmt.Sprintf("The %s field must be equal to %s.", field, param)

	case "ne":
		return fmt.Sprintf("The %s field must not be equal to %s.", field, param)

	case "gt":
		return fmt.Sprintf("The %s field must be greater than %s.", field, param)

	case "gte":
		return fmt.Sprintf("The %s field must be greater than or equal to %s.", field, param)

	case "lt":
		return fmt.Sprintf("The %s field must be less than %s.", field, param)

	case "lte":
		return fmt.Sprintf("The %s field must be less than or equal to %s.", field, param)

	case "eq_ignore_case":
		return fmt.Sprintf("The %s field must be equal to %s, ignoring case.", field, param)

	case "ne_ignore_case":
		return fmt.Sprintf("The %s field must not be equal to %s, ignoring case.", field, param)

	// Comparisons against another field of the same struct.
	case "eqfield":
		return fmt.Sprintf("The %s field must match the %s field.", field, fieldLabel(param))

	case "nefield":
		return fmt.Sprintf("The %s field must be different from the %s field.", field, fieldLabel(param))

	case "gtfield":
		return fmt.Sprintf("The %s field must be greater than the %s field.", field, fieldLabel(param))

	case "gtefield":
		return fmt.Sprintf("The %s field must be greater than or equal to the %s field.", field, fieldLabel(param))

	case "ltfield":
		return fmt.Sprintf("The %s field must be less than the %s field.", field, fieldLabel(param))

	case "ltefield":
		return fmt.Sprintf("The %s field must be less than or equal to the %s field.", field, fieldLabel(param))

	case "fieldcontains":
		return fmt.Sprintf("The %s field must contain the %s field.", field, fieldLabel(param))

	case "fieldexcludes":
		return fmt.Sprintf("The %s field must not contain the %s field.", field, fieldLabel(param))

	// Comparisons against another field reached by a relative path.
	case "eqcsfield":
		return fmt.Sprintf("The %s field must match the %s field.", field, fieldLabel(param))

	case "necsfield":
		return fmt.Sprintf("The %s field must be different from the %s field.", field, fieldLabel(param))

	case "gtcsfield":
		return fmt.Sprintf("The %s field must be greater than the %s field.", field, fieldLabel(param))

	case "gtecsfield":
		return fmt.Sprintf("The %s field must be greater than or equal to the %s field.", field, fieldLabel(param))

	case "ltcsfield":
		return fmt.Sprintf("The %s field must be less than the %s field.", field, fieldLabel(param))

	case "ltecsfield":
		return fmt.Sprintf("The %s field must be less than or equal to the %s field.", field, fieldLabel(param))

	// Size.
	case "len":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("The %s field must be exactly %s characters.", field, param)
		}

		if isCollection(fe.Kind()) {
			return fmt.Sprintf("The %s field must contain exactly %s items.", field, param)
		}

		return fmt.Sprintf("The %s field must be exactly %s.", field, param)

	case "min":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("The %s field must be at least %s characters.", field, param)
		}

		if isCollection(fe.Kind()) {
			return fmt.Sprintf("The %s field must contain at least %s items.", field, param)
		}

		return fmt.Sprintf("The %s field must be at least %s.", field, param)

	case "max":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("The %s field must not be greater than %s characters.", field, param)
		}

		if isCollection(fe.Kind()) {
			return fmt.Sprintf("The %s field must not contain more than %s items.", field, param)
		}

		return fmt.Sprintf("The %s field must not be greater than %s.", field, param)

	case "oneof":
		return fmt.Sprintf("The selected %s is invalid.", field)

	case "noneof":
		return fmt.Sprintf("The selected %s is invalid.", field)

	// Scalars.
	case "boolean":
		return fmt.Sprintf("The %s field must be true or false.", field)

	case "number":
		return fmt.Sprintf("The %s field must be a number.", field)

	case "numeric":
		return fmt.Sprintf("The %s field must be numeric.", field)

	case "integer":
		return fmt.Sprintf("The %s field must be an integer.", field)

	case "isdefault":
		return fmt.Sprintf("The %s field must be the default value.", field)

	case "unique":
		return fmt.Sprintf("The %s field has already been taken.", field)

	// Strings.
	case "alpha":
		return fmt.Sprintf("The %s field must only contain letters.", field)

	case "alphaspace":
		return fmt.Sprintf("The %s field must only contain letters and spaces.", field)

	case "alphaunicode":
		return fmt.Sprintf("The %s field must only contain letters and unicode characters.", field)

	case "alphanum":
		return fmt.Sprintf("The %s field must only contain letters and numbers.", field)

	case "alphanumspace":
		return fmt.Sprintf("The %s field must only contain letters, numbers and spaces.", field)

	case "alphanumunicode":
		return fmt.Sprintf("The %s field must only contain letters, numbers and unicode characters.", field)

	case "ascii":
		return fmt.Sprintf("The %s field must only contain single byte characters.", field)

	case "printascii":
		return fmt.Sprintf("The %s field must only contain printable ASCII characters.", field)

	case "multibyte":
		return fmt.Sprintf("The %s field must contain at least one multi byte character.", field)

	case "lowercase":
		return fmt.Sprintf("The %s field must be lowercase.", field)

	case "uppercase":
		return fmt.Sprintf("The %s field must be uppercase.", field)

	case "contains":
		return fmt.Sprintf("The %s field must contain %s.", field, param)

	case "containsany":
		return fmt.Sprintf("The %s field must contain at least one of %s.", field, param)

	case "containsrune":
		return fmt.Sprintf("The %s field must contain the character %s.", field, param)

	case "excludes":
		return fmt.Sprintf("The %s field must not contain %s.", field, param)

	case "excludesall":
		return fmt.Sprintf("The %s field must not contain any of %s.", field, param)

	case "excludesrune":
		return fmt.Sprintf("The %s field must not contain the character %s.", field, param)

	case "startswith":
		return fmt.Sprintf("The %s field must start with %s.", field, param)

	case "startsnotwith":
		return fmt.Sprintf("The %s field must not start with %s.", field, param)

	case "endswith":
		return fmt.Sprintf("The %s field must end with %s.", field, param)

	case "endsnotwith":
		return fmt.Sprintf("The %s field must not end with %s.", field, param)

	// Formats and codes.
	case "email":
		return fmt.Sprintf("The %s field must be a valid email address.", field)

	case "url":
		return fmt.Sprintf("The %s field must be a valid URL.", field)

	case "http_url":
		return fmt.Sprintf("The %s field must be a valid HTTP URL.", field)

	case "https_url":
		return fmt.Sprintf("The %s field must be a valid HTTPS URL.", field)

	case "origin":
		return fmt.Sprintf("The %s field must be a valid web origin.", field)

	case "uri":
		return fmt.Sprintf("The %s field must be a valid URI.", field)

	case "urn_rfc2141":
		return fmt.Sprintf("The %s field must be a valid URN.", field)

	case "urn_rfc8141":
		return fmt.Sprintf("The %s field must be a valid URN.", field)

	case "datauri":
		return fmt.Sprintf("The %s field must be a valid data URL.", field)

	case "url_encoded":
		return fmt.Sprintf("The %s field must be URL encoded.", field)

	case "html":
		return fmt.Sprintf("The %s field must not contain HTML tags.", field)

	case "html_encoded":
		return fmt.Sprintf("The %s field must be HTML encoded.", field)

	case "json":
		return fmt.Sprintf("The %s field must be a valid JSON string.", field)

	case "jwt":
		return fmt.Sprintf("The %s field must be a valid JWT.", field)

	case "base64":
		return fmt.Sprintf("The %s field must be a valid base64 string.", field)

	case "base64url":
		return fmt.Sprintf("The %s field must be a valid base64 URL string.", field)

	case "base64rawurl":
		return fmt.Sprintf("The %s field must be a valid base64 raw URL string.", field)

	case "datetime":
		return fmt.Sprintf("The %s field must be a valid datetime.", field)

	case "duration":
		return fmt.Sprintf("The %s field must be a valid duration.", field)

	case "timezone":
		return fmt.Sprintf("The %s field must be a valid timezone.", field)

	case "semver":
		return fmt.Sprintf("The %s field must be a valid semantic version.", field)

	case "ulid":
		return fmt.Sprintf("The %s field must be a valid ULID.", field)

	case "cve":
		return fmt.Sprintf("The %s field must be a valid CVE id.", field)

	case "cron":
		return fmt.Sprintf("The %s field must be a valid cron expression.", field)

	case "latitude":
		return fmt.Sprintf("The %s field must be a valid latitude.", field)

	case "longitude":
		return fmt.Sprintf("The %s field must be a valid longitude.", field)

	case "luhn_checksum":
		return fmt.Sprintf("The %s field must pass the Luhn checksum.", field)

	case "credit_card":
		return fmt.Sprintf("The %s field must be a valid credit card number.", field)

	case "e164":
		return fmt.Sprintf("The %s field must be a valid phone number.", field)

	case "ssn":
		return fmt.Sprintf("The %s field must be a valid social security number.", field)

	case "ein":
		return fmt.Sprintf("The %s field must be a valid EIN.", field)

	case "isbn":
		return fmt.Sprintf("The %s field must be a valid ISBN.", field)

	case "isbn10":
		return fmt.Sprintf("The %s field must be a valid ISBN 10.", field)

	case "isbn13":
		return fmt.Sprintf("The %s field must be a valid ISBN 13.", field)

	case "issn":
		return fmt.Sprintf("The %s field must be a valid ISSN.", field)

	case "iso3166_1_alpha2":
		return fmt.Sprintf("The %s field must be a valid two letter country code.", field)

	case "iso3166_1_alpha3":
		return fmt.Sprintf("The %s field must be a valid three letter country code.", field)

	case "iso3166_1_alpha_numeric":
		return fmt.Sprintf("The %s field must be a valid numeric country code.", field)

	case "iso3166_2":
		return fmt.Sprintf("The %s field must be a valid country subdivision code.", field)

	case "iso4217":
		return fmt.Sprintf("The %s field must be a valid currency code.", field)

	case "country_code":
		return fmt.Sprintf("The %s field must be a valid country code.", field)

	case "bcp47_language_tag":
		return fmt.Sprintf("The %s field must be a valid language tag.", field)

	case "bcp47_strict_language_tag":
		return fmt.Sprintf("The %s field must be a valid language tag.", field)

	case "postcode_iso3166_alpha2":
		return fmt.Sprintf("The %s field must be a valid postcode.", field)

	case "postcode_iso3166_alpha2_field":
		return fmt.Sprintf("The %s field must be a valid postcode.", field)

	case "bic":
		return fmt.Sprintf("The %s field must be a valid BIC.", field)

	case "bic_iso_9362_2014":
		return fmt.Sprintf("The %s field must be a valid BIC.", field)

	case "btc_addr":
		return fmt.Sprintf("The %s field must be a valid bitcoin address.", field)

	case "btc_addr_bech32":
		return fmt.Sprintf("The %s field must be a valid bitcoin address.", field)

	case "eth_addr":
		return fmt.Sprintf("The %s field must be a valid ethereum address.", field)

	case "mongodb":
		return fmt.Sprintf("The %s field must be a valid MongoDB ObjectID.", field)

	case "mongodb_connection_string":
		return fmt.Sprintf("The %s field must be a valid MongoDB connection string.", field)

	case "spicedb":
		return fmt.Sprintf("The %s field must be a valid SpiceDB id.", field)

	case "hexcolor":
		return fmt.Sprintf("The %s field must be a valid hex color.", field)

	case "rgb":
		return fmt.Sprintf("The %s field must be a valid RGB color.", field)

	case "rgba":
		return fmt.Sprintf("The %s field must be a valid RGBA color.", field)

	case "hsl":
		return fmt.Sprintf("The %s field must be a valid HSL color.", field)

	case "hsla":
		return fmt.Sprintf("The %s field must be a valid HSLA color.", field)

	case "cmyk":
		return fmt.Sprintf("The %s field must be a valid CMYK color.", field)

	case "iscolor":
		return fmt.Sprintf("The %s field must be a valid color.", field)

	case "hexadecimal":
		return fmt.Sprintf("The %s field must be a hexadecimal string.", field)

	case "md4":
		return fmt.Sprintf("The %s field must be a valid MD4 hash.", field)

	case "md5":
		return fmt.Sprintf("The %s field must be a valid MD5 hash.", field)

	case "sha256":
		return fmt.Sprintf("The %s field must be a valid SHA256 hash.", field)

	case "sha384":
		return fmt.Sprintf("The %s field must be a valid SHA384 hash.", field)

	case "sha512":
		return fmt.Sprintf("The %s field must be a valid SHA512 hash.", field)

	case "ripemd128":
		return fmt.Sprintf("The %s field must be a valid RIPEMD128 hash.", field)

	case "ripemd160":
		return fmt.Sprintf("The %s field must be a valid RIPEMD160 hash.", field)

	case "tiger128":
		return fmt.Sprintf("The %s field must be a valid TIGER128 hash.", field)

	case "tiger160":
		return fmt.Sprintf("The %s field must be a valid TIGER160 hash.", field)

	case "tiger192":
		return fmt.Sprintf("The %s field must be a valid TIGER192 hash.", field)

	case "uuid":
		return fmt.Sprintf("The %s field must be a valid UUID.", field)

	case "uuid_rfc4122":
		return fmt.Sprintf("The %s field must be a valid UUID.", field)

	case "uuid3":
		return fmt.Sprintf("The %s field must be a valid UUID v3.", field)

	case "uuid3_rfc4122":
		return fmt.Sprintf("The %s field must be a valid UUID v3.", field)

	case "uuid4":
		return fmt.Sprintf("The %s field must be a valid UUID v4.", field)

	case "uuid4_rfc4122":
		return fmt.Sprintf("The %s field must be a valid UUID v4.", field)

	case "uuid5":
		return fmt.Sprintf("The %s field must be a valid UUID v5.", field)

	case "uuid5_rfc4122":
		return fmt.Sprintf("The %s field must be a valid UUID v5.", field)

	// Network addresses.
	case "ip":
		return fmt.Sprintf("The %s field must be a valid IP address.", field)

	case "ip_addr":
		return fmt.Sprintf("The %s field must be a valid IP address.", field)

	case "ipv4":
		return fmt.Sprintf("The %s field must be a valid IPv4 address.", field)

	case "ip4_addr":
		return fmt.Sprintf("The %s field must be a valid IPv4 address.", field)

	case "ipv6":
		return fmt.Sprintf("The %s field must be a valid IPv6 address.", field)

	case "ip6_addr":
		return fmt.Sprintf("The %s field must be a valid IPv6 address.", field)

	case "cidr":
		return fmt.Sprintf("The %s field must be a valid CIDR.", field)

	case "cidrv4":
		return fmt.Sprintf("The %s field must be a valid IPv4 CIDR.", field)

	case "cidrv6":
		return fmt.Sprintf("The %s field must be a valid IPv6 CIDR.", field)

	case "mac":
		return fmt.Sprintf("The %s field must be a valid MAC address.", field)

	case "port":
		return fmt.Sprintf("The %s field must be a valid port number.", field)

	case "hostname":
		return fmt.Sprintf("The %s field must be a valid hostname.", field)

	case "hostname_rfc1123":
		return fmt.Sprintf("The %s field must be a valid hostname.", field)

	case "hostname_port":
		return fmt.Sprintf("The %s field must be a valid host and port.", field)

	case "fqdn":
		return fmt.Sprintf("The %s field must be a valid fully qualified domain name.", field)

	case "tcp_addr":
		return fmt.Sprintf("The %s field must be a valid TCP address.", field)

	case "tcp4_addr":
		return fmt.Sprintf("The %s field must be a valid IPv4 TCP address.", field)

	case "tcp6_addr":
		return fmt.Sprintf("The %s field must be a valid IPv6 TCP address.", field)

	case "udp_addr":
		return fmt.Sprintf("The %s field must be a valid UDP address.", field)

	case "udp4_addr":
		return fmt.Sprintf("The %s field must be a valid IPv4 UDP address.", field)

	case "udp6_addr":
		return fmt.Sprintf("The %s field must be a valid IPv6 UDP address.", field)

	case "unix_addr":
		return fmt.Sprintf("The %s field must be a valid unix socket address.", field)

	case "uds_exists":
		return fmt.Sprintf("The %s field must point to an existing unix socket.", field)

	// Files and media.
	case "file":
		return fmt.Sprintf("The %s field must be an existing file.", field)

	case "filepath":
		return fmt.Sprintf("The %s field must be a valid file path.", field)

	case "dir":
		return fmt.Sprintf("The %s field must be an existing directory.", field)

	case "dirpath":
		return fmt.Sprintf("The %s field must be a valid directory path.", field)

	case "image":
		return fmt.Sprintf("The %s field must be an image.", field)

	case "mimetype":
		return fmt.Sprintf("The %s field must be a valid MIME type.", field)

	case "validateFn":
		return fmt.Sprintf("The %s field is invalid.", field)

	default:
		return fmt.Sprintf("The %s field is invalid.", field)
	}
}

func fieldLabel(name string) string {
	runes := []rune(name)
	parts := make([]string, 0, len(runes))

	for i, r := range runes {
		afterLower := i > 0 && runes[i-1] >= 'a' && runes[i-1] <= 'z'
		beforeLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'

		if i > 0 && r >= 'A' && r <= 'Z' && (afterLower || beforeLower) {
			parts = append(parts, " ")
		}

		parts = append(parts, string(r))
	}

	return strings.ToLower(strings.Join(parts, ""))
}

func fieldParam(param string) (string, string) {
	name, value, _ := strings.Cut(param, " ")

	return fieldLabel(name), value
}

func isCollection(kind reflect.Kind) bool {
	return kind == reflect.Slice || kind == reflect.Array || kind == reflect.Map
}
