package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jlry-dev/whirl/internal/config"
	"github.com/jlry-dev/whirl/internal/handler"
	"github.com/jlry-dev/whirl/internal/util"
)

type Middleware interface {
	Authenticator(next http.HandlerFunc) http.HandlerFunc
	CorsMiddleware(next http.Handler) http.Handler
}

type middlewareStruct struct {
	rsp     *handler.ResponseHandler
	logger  *slog.Logger
	policy  *config.OriginPolicy
	tickets *handler.TicketStore
	jwtKey  []byte
}

func NewMiddleware(rsp *handler.ResponseHandler, logger *slog.Logger, policy *config.OriginPolicy, tickets *handler.TicketStore, jwtKey []byte) Middleware {
	return &middlewareStruct{
		rsp:     rsp,
		logger:  logger,
		policy:  policy,
		tickets: tickets,
		jwtKey:  jwtKey,
	}
}

func (m *middlewareStruct) CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		m.logger.Info("request has been made", slog.String("origin", origin))

		// CORS only applies kung naay Origin header. Non-browser clients
		// (curl, health checks) wala man, so dili dapat i-reject. OPTIONS
		// is answered here kay ang ServeMux mo 405 sa preflight.
		if origin == "" {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if !m.policy.Allows(origin) {
			// No CORS headers, para ma block sa browser.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.Error(w, "CORS origin not allowed", http.StatusForbidden)
			return
		}

		// Echo the exact origin. Never "*", ug ayaw ibutang ang configured
		// value kay ang shared cache dapat mo key sa request origin.
		w.Header().Set("Access-Control-Allow-Origin", origin)
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

// websocketConnectPath is the only route nga pwede mo auth via URL,
// kay dili man kabutang header ang browser sa WebSocket handshake.
const websocketConnectPath = "/websocket/connect"

// userIDKey is the context key used by the handlers.
// String lang ni for historical reasons; changing it would touch every handler.
const userIDKey = "userID"

func (m *middlewareStruct) Authenticator(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		// If naa Authorization header, dapat valid jud. Ayaw fallback sa
		// query, kay mao na ang rason nga naay JWT sa URL before.
		if authHeader != "" {
			parts := strings.Fields(authHeader)
			if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
				m.logger.Error("auth: malformed authorization header", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
				m.rsp.Error(w, http.StatusUnauthorized, "invalid token", nil)
				return
			}

			m.authenticate(w, r, parts[1], next)
			return
		}

		// WebSocket ra ang pwede mo auth via URL, kay dili man kabutang
		// header ang browser sa handshake. Other routes, header lang jud.
		if r.URL.Path != websocketConnectPath {
			m.rsp.Error(w, http.StatusUnauthorized, "invalid token", nil)
			return
		}

		// Ticket na lang. Single use ni and 30s ra, so wala nay gamit
		// kung ma-log ang URL.
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" {
			m.rsp.Error(w, http.StatusUnauthorized, "invalid ticket", nil)
			return
		}

		userID, ok := m.tickets.Consume(ticket)
		if !ok {
			m.rsp.Error(w, http.StatusUnauthorized, "invalid ticket", nil)
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	}
}

func (m *middlewareStruct) authenticate(w http.ResponseWriter, r *http.Request, tokenStr string, next http.HandlerFunc) {
	ctx := r.Context()

	// parse the claims of the token
	token, err := util.ParseJWT(ctx, m.jwtKey, tokenStr)
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

	next(w, r.WithContext(context.WithValue(ctx, userIDKey, userID)))
}
