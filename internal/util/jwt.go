package util

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 24h ra ang token. Wala pa tay revocation, so ang expiry ra ang nag-limit
// kung unsa kadugay magamit ang stolen token.
const (
	TokenTTL      = 24 * time.Hour
	tokenIssuer   = "whirl"
	tokenAudience = "whirl-api"
)

var ErrMissingJWTKey = errors.New("util: JWT key is empty")

func GenerateJWT(ctx context.Context, key []byte, subject int) (string, error) {
	if len(key) == 0 {
		return "", ErrMissingJWTKey
	}

	now := time.Now()

	claims := jwt.RegisteredClaims{
		Issuer:    tokenIssuer,
		Subject:   strconv.Itoa(subject),
		Audience:  jwt.ClaimStrings{tokenAudience},
		ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		IssuedAt:  jwt.NewNumericDate(now),
	}

	uToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := uToken.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("util: failed to sign token : %w", err)
	}

	return token, nil
}

/*
This helper function parses the given token string

Returns the token struct
*/
func ParseJWT(ctx context.Context, key []byte, token string) (*jwt.Token, error) {
	if len(key) == 0 {
		return nil, ErrMissingJWTKey
	}

	t, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return key, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithAudience(tokenAudience),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return nil, err
	}

	return t, nil
}
