package config

import (
	"log/slog"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/jlry-dev/whirl/internal/util"
)

type Config struct {
	Logger   *slog.Logger
	Validate *validator.Validate
}

func Load() Config {
	l := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}))

	// Register validator
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterValidation("age", util.ValidAgeValidator)
	v.RegisterValidation("dateformat", util.DateFormatValidator)

	return Config{
		Logger:   l,
		Validate: v,
	}
}
