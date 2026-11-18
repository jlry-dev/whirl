package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/jlry-dev/whirl/internal/util"
)

// jwtMinKeyBytes is the minimum HS256 key size. HS256 output is 32 bytes, so a
// shorter key weakens the signature more than the hash itself.
const jwtMinKeyBytes = 32

type Config struct {
	Logger   *slog.Logger
	Validate *validator.Validate
	Origins  *OriginPolicy
	JWTKey   []byte
}

func Load() (Config, error) {
	l := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}))

	// Register validator
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterValidation("age", util.ValidAgeValidator)
	v.RegisterValidation("dateformat", util.DateFormatValidator)
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	key, err := loadJWTKey()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Logger:   l,
		Validate: v,
		Origins:  LoadOriginPolicy(l),
		JWTKey:   key,
	}, nil
}

// loadJWTKey reads JWT_KEY once at startup. Daghan kaayo ug paagi ma-misconfigure
// ni, so fail fast na lang: kung wala or mubo ang key, dili mo-start ang server.
func loadJWTKey() ([]byte, error) {
	key := strings.TrimSpace(os.Getenv("JWT_KEY"))
	if key == "" {
		return nil, errors.New("config: JWT_KEY is required")
	}

	if len(key) < jwtMinKeyBytes {
		return nil, fmt.Errorf("config: JWT_KEY must be at least %d bytes, got %d (try: openssl rand -base64 48)", jwtMinKeyBytes, len(key))
	}

	return []byte(key), nil
}
