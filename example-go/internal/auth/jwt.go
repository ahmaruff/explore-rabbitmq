package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

type Claims struct {
	jwt.RegisteredClaims
}

func (c *Claims) UnmarshalJSON(data []byte) error {
	type Alias jwt.RegisteredClaims

	var raw struct {
		Subject json.RawMessage `json:"sub"`
		*Alias
	}

	raw.Alias = (*Alias)(&c.RegisteredClaims)

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if len(raw.Subject) == 0 {
		return errors.New("missing subject")
	}

	var subject string
	if err := json.Unmarshal(raw.Subject, &subject); err != nil {
		var number json.Number
		if err := json.Unmarshal(raw.Subject, &number); err != nil {
			return errors.New("invalid subject")
		}

		subject = number.String()
	}

	c.Subject = subject
	return nil
}

func NewService(secret string, issuer string, ttl time.Duration) *JWTService {
	return &JWTService{
		secret: []byte(secret),
		issuer: issuer,
		ttl:    ttl,
	}
}

func (s *JWTService) Generate(userID string) (string, error) {
	now := time.Now()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(s.secret)
}

func (s *JWTService) Validate(tokenString string) (*Claims, error) {
	options := []jwt.ParserOption{
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30 * time.Second),
	}

	if s.issuer != "" {
		options = append(options, jwt.WithIssuer(s.issuer))
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf(
					"unsupported signing algorithm %q: only HS256 is verified with the shared secret",
					token.Method.Alg(),
				)
			}

			return s.secret, nil
		},
		options...,
	)

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("parsed token carries an unexpected claims type")
	}

	if !token.Valid {
		return nil, errors.New("token is not valid")
	}

	if claims.Subject == "" {
		return nil, errors.New("token has no subject claim (sub)")
	}

	return claims, nil

}
