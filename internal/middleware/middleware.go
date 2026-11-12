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
	allowedOrigins := parseAllowedOrigins(os.Getenv("FRONTEND_ADDRESS"))
	allowAll := len(allowedOrigins) == 0

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		m.logger.Info("request has been made", slog.String("origin", origin))

		// CORS only applies to requests that carry an Origin header. Non-browser
		// clients (curl, health checks, server-to-server) send none and must not
		// be rejected. OPTIONS is still answered here because Go's method-based
		// ServeMux patterns would otherwise return 405 for preflight requests.
		if origin == "" {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		allowedOrigin, ok := resolveOrigin(origin, allowedOrigins, allowAll)
		if !ok {
			// Add no CORS headers so the browser blocks the response.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.Error(w, "CORS origin not allowed", http.StatusForbidden)
			return
		}

		// Set the header exactly once. Setting it again later would overwrite the
		// value computed here, which is how the previous implementation broke
		// allow-all mode.
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		// Reflected origins make the response vary by Origin, so shared caches
		// must key on it.
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// parseAllowedOrigins splits a comma-separated FRONTEND_ADDRESS into a set of
// allowed origins. An empty value means "allow any origin".
func parseAllowedOrigins(raw string) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(part)
		// The Origin header never includes a trailing slash, so normalize the
		// configured value instead of the request.
		origin = strings.TrimRight(origin, "/")
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return allowed
}

// resolveOrigin returns the value for Access-Control-Allow-Origin and whether
// the request origin is permitted.
func resolveOrigin(origin string, allowed map[string]struct{}, allowAll bool) (string, bool) {
	if allowAll {
		return "*", true
	}
	if _, ok := allowed[origin]; ok {
		return origin, true
	}
	return "", false
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
			return
		}

		nCtx := context.WithValue(ctx, "userID", userID)
		r2 := r.WithContext(nCtx)
		next(w, r2)
	}
}
