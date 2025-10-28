package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jlry-dev/whirl/internal/handler"
	"github.com/jlry-dev/whirl/internal/util"
)

type Middleware interface {
	Authenticator(next http.HandlerFunc) http.HandlerFunc
}

type middlewareStruct struct {
	rsp    *handler.ResponseHandler
	logger *slog.Logger
}

func NewMiddleware(rsp *handler.ResponseHandler, logger *slog.Logger) Middleware {
	return &middlewareStruct{
		rsp:    rsp,
		logger: logger,
	}
}

func (m *middlewareStruct) Authenticator(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		authHeader := r.Header.Get("Authorization")
		bearerSlice := strings.Fields(authHeader)

		if len(bearerSlice) < 2 || len(bearerSlice) > 2 || bearerSlice[0] != "Bearer" {
			m.rsp.Error(w, http.StatusUnauthorized, "invalid token")
			return
		}

		// parse the claims of the token
		token, err := util.ParseJWT(ctx, bearerSlice[1])
		if err != nil {
			m.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
			if errors.Is(err, jwt.ErrTokenExpired) {
				m.rsp.Error(w, http.StatusUnauthorized, "token expired")
				return
			}
			m.rsp.Error(w, http.StatusUnauthorized, "invalid token")
			return
		}

		sub, err := token.Claims.GetSubject()
		if err != nil {
			m.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
			m.rsp.Error(w, http.StatusUnauthorized, "invalid token")
			return
		}

		userID, err := strconv.Atoi(sub)
		if err != nil {
			m.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
			m.rsp.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}

		nCtx := context.WithValue(ctx, "userID", userID)
		r2 := r.WithContext(nCtx)
		next(w, r2)
	}
}
