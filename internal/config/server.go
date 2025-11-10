package config

import (
	"log/slog"
	"os"
	"reflect"
	"strings"

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
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	return Config{
		Logger:   l,
		Validate: v,
	}
}
