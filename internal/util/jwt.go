package util

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateJWT(ctx context.Context, subject int) (string, error) {
	var key []byte

	if k := os.Getenv("JWT_KEY"); k == "" {
		panic("JWT_KEY env var is missing")
	} else {
		key = []byte(k)
	}

	claims := jwt.RegisteredClaims{
		Subject:   strconv.Itoa(subject),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(14 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
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
func ParseJWT(ctx context.Context, token string) (*jwt.Token, error) {
	t, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return []byte(os.Getenv("JWT_KEY")), nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		return new(jwt.Token), err
	}

	return t, nil
}
