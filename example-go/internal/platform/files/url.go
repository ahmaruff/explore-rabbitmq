package files

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"rabbitmq-example/internal/platform/storage"
)

const RoutePrefix = "/files"

// RoutePattern is the router pattern for the handler.
const RoutePattern = RoutePrefix + "/*"

const defaultSignedURLTTL = 15 * time.Minute

var ErrSigningUnavailable = errors.New("signed URLs are not supported by this storage driver")

func (s *Service) URL(key string) string {
	relative := RoutePrefix + "/" + strings.TrimPrefix(key, "/")

	if s.baseURL == "" {
		return relative
	}

	return s.baseURL + relative
}

func (s *Service) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	key = strings.TrimPrefix(key, "/")

	// ini kalo pakai signer yang diprovide oleh storage impl (S3)
	if signer, ok := s.store.(storage.Signer); ok {
		return signer.SignedURL(ctx, key, ttl)
	}

	if len(s.signingKey) == 0 {
		return "", ErrSigningUnavailable
	}

	if ttl <= 0 {
		ttl = defaultSignedURLTTL
	}

	// ini kalo signing sendiri
	return s.signedURL(key, time.Now().Add(ttl)), nil
}

// signedURL builds the local form of a signed link.
func (s *Service) signedURL(key string, expires time.Time) string {
	timestamp := expires.Unix()

	return s.URL(key) +
		"?expires=" + strconv.FormatInt(timestamp, 10) +
		"&signature=" + signatureFor(key, timestamp, s.signingKey)
}

// signatureFor signs the key and the expiry, so neither can be changed without
// invalidating the link. The route prefix is left out on purpose: moving the
// handler must not break links that were already handed out.
func signatureFor(key string, expires int64, signingKey []byte) string {
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(key))
	mac.Write([]byte("\n"))
	mac.Write([]byte(strconv.FormatInt(expires, 10)))

	return hex.EncodeToString(mac.Sum(nil))
}
