package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jlry-dev/whirl/internal/handler"
	"github.com/jlry-dev/whirl/internal/util"
)

type Middleware interface {
	Authenticator(next http.HandlerFunc) http.HandlerFunc
	CorsMiddleware(next http.Handler) http.Handler
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

func (m *middlewareStruct) CorsMiddleware(next http.Handler) http.Handler {
	frontendURL := os.Getenv("FRONTEND_ADDRESS")
	allowAll := frontendURL == ""

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		m.logger.Info("request has been made", slog.String("origin", origin))

		if allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin == frontendURL {
			w.Header().Set("Access-Control-Allow-Origin", origin) // Reflect for security
		} else {
			http.Error(w, "CORS origin not allowed", http.StatusForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", frontendURL)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *middlewareStruct) Authenticator(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var tokenStr string

		authHeader := r.Header.Get("Authorization")
		bearerSlice := strings.Fields(authHeader)

		if len(bearerSlice) < 2 || len(bearerSlice) > 2 || bearerSlice[0] != "Bearer" {
			// Check for websockets fallback
			wsToken := r.URL.Query().Get("token")

			if wsToken == "" {
				m.rsp.Error(w, http.StatusUnauthorized, "invalid token", nil)
				return
			}

			tokenStr = wsToken
		} else {
			tokenStr = bearerSlice[1]
		}

		// parse the claims of the token
		token, err := util.ParseJWT(ctx, tokenStr)
		if err != nil {
			m.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
			if errors.Is(err, jwt.ErrTokenExpired) {
				m.rsp.Error(w, http.StatusUnauthorized, "token expired", nil)
				return
			}
			m.rsp.Error(w, http.StatusUnauthorized, "invalid token", nil)
			return
		}

		sub, err := token.Claims.GetSubject()
		if err != nil {
			m.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
			m.rsp.Error(w, http.StatusUnauthorized, "invalid token", nil)
			return
		}

		userID, err := strconv.Atoi(sub)
		if err != nil {
			m.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
			m.rsp.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		}

		nCtx := context.WithValue(ctx, "userID", userID)
		r2 := r.WithContext(nCtx)
		next(w, r2)
	}
}
