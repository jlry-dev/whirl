package middleware_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jlry-dev/whirl/internal/config"
	"github.com/jlry-dev/whirl/internal/handler"
	"github.com/jlry-dev/whirl/internal/middleware"
	"github.com/jlry-dev/whirl/internal/util"
)

const (
	frontendOrigin = "https://whirl.example"
	testJWTKey     = "test-key-at-least-32-bytes-long-1234"
)

func newTestMiddleware(t *testing.T, store *handler.TicketStore) middleware.Middleware {
	t.Helper()

	t.Setenv("FRONTEND_ADDRESS", frontendOrigin)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rsp := handler.NewResponseHandler(logger)
	policy := config.LoadOriginPolicy(logger)

	if store == nil {
		store = handler.NewTicketStore(time.Minute)
	}

	return middleware.NewMiddleware(rsp, logger, policy, store, []byte(testJWTKey))
}

func newToken(t *testing.T, userID int) string {
	t.Helper()

	token, err := util.GenerateJWT(context.Background(), []byte(testJWTKey), userID)
	require.NoError(t, err)

	return token
}

// authenticate runs one request through the authenticator and returns the
// status code, if the handler ran, and the userID it saw.
func authenticate(m middleware.Middleware, target string, headers map[string]string) (int, bool, int) {
	called := false
	gotUser := 0

	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotUser, _ = r.Context().Value("userID").(int)
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequest(http.MethodGet, target, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	m.Authenticator(next)(rec, req)

	return rec.Code, called, gotUser
}

func Test_Authenticator_AcceptsBearerHeader(t *testing.T) {
	m := newTestMiddleware(t, nil)
	token := newToken(t, 7)

	code, called, userID := authenticate(m, "/friends", map[string]string{
		"Authorization": "Bearer " + token,
	})

	assert.Equal(t, http.StatusOK, code)
	assert.True(t, called)
	assert.Equal(t, 7, userID)
}

func Test_Authenticator_RejectsQueryTokenOnREST(t *testing.T) {
	m := newTestMiddleware(t, nil)
	token := newToken(t, 7)

	code, called, _ := authenticate(m, "/friends?token="+token, nil)

	assert.Equal(t, http.StatusUnauthorized, code, "query token wala na gi-accept sa bisan unsa nga route")
	assert.False(t, called)
}

func Test_Authenticator_RejectsQueryJWTOnWebsocket(t *testing.T) {
	m := newTestMiddleware(t, nil)
	token := newToken(t, 7)

	// Frontend is already using the ticket flow, so balik na lang ta 401
	// kung naay mo try ug butang sa JWT sa URL.
	code, called, _ := authenticate(m, "/websocket/connect?token="+token, nil)

	assert.Equal(t, http.StatusUnauthorized, code)
	assert.False(t, called)
}

func Test_Authenticator_AcceptsTicketOnWebsocket(t *testing.T) {
	store := handler.NewTicketStore(time.Minute)
	m := newTestMiddleware(t, store)

	ticket, err := store.Issue(11)
	require.NoError(t, err)

	code, called, userID := authenticate(m, "/websocket/connect?ticket="+ticket, nil)

	assert.Equal(t, http.StatusOK, code)
	assert.True(t, called)
	assert.Equal(t, 11, userID)
}

func Test_Authenticator_RejectsReusedTicket(t *testing.T) {
	store := handler.NewTicketStore(time.Minute)
	m := newTestMiddleware(t, store)

	ticket, err := store.Issue(11)
	require.NoError(t, err)

	code, called, _ := authenticate(m, "/websocket/connect?ticket="+ticket, nil)
	require.Equal(t, http.StatusOK, code)
	require.True(t, called)

	code, called, _ = authenticate(m, "/websocket/connect?ticket="+ticket, nil)

	assert.Equal(t, http.StatusUnauthorized, code)
	assert.False(t, called)
}

func Test_Authenticator_MalformedHeaderDoesNotFallBackToTicket(t *testing.T) {
	store := handler.NewTicketStore(time.Minute)
	m := newTestMiddleware(t, store)
	token := newToken(t, 7)

	// Valid ni nga ticket, pero dapat ma-reject gihapon kay naa malformed
	// Authorization header. No fallback jud.
	ticket, err := store.Issue(11)
	require.NoError(t, err)

	cases := map[string]string{
		"basic scheme":   "Basic dXNlcjpwYXNz",
		"missing value":  "Bearer",
		"too many parts": "Bearer " + token + " extra",
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			code, called, _ := authenticate(m, "/websocket/connect?ticket="+ticket, map[string]string{
				"Authorization": header,
			})

			assert.Equal(t, http.StatusUnauthorized, code)
			assert.False(t, called)
		})
	}
}

func Test_Authenticator_RejectsMissingCredentials(t *testing.T) {
	m := newTestMiddleware(t, nil)

	code, called, _ := authenticate(m, "/friends", nil)

	assert.Equal(t, http.StatusUnauthorized, code)
	assert.False(t, called)
}

func Test_Authenticator_RejectsEmptyTicket(t *testing.T) {
	m := newTestMiddleware(t, nil)

	code, called, _ := authenticate(m, "/websocket/connect?ticket=", nil)

	assert.Equal(t, http.StatusUnauthorized, code)
	assert.False(t, called)
}

func Test_CorsMiddleware_EchoesAllowedOrigin(t *testing.T) {
	m := newTestMiddleware(t, nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/friends", nil)
	req.Header.Set("Origin", frontendOrigin)

	rec := httptest.NewRecorder()
	m.CorsMiddleware(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, frontendOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", rec.Header().Get("Vary"))
}

func Test_CorsMiddleware_RejectsUnknownOrigin(t *testing.T) {
	m := newTestMiddleware(t, nil)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/friends", nil)
	req.Header.Set("Origin", "https://evil.example")

	rec := httptest.NewRecorder()
	m.CorsMiddleware(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.False(t, called)
}

func Test_CorsMiddleware_AllowsRequestsWithoutOrigin(t *testing.T) {
	m := newTestMiddleware(t, nil)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	rec := httptest.NewRecorder()
	m.CorsMiddleware(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
}
