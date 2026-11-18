package util_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jlry-dev/whirl/internal/util"
)

var (
	testKey  = []byte("test-key-at-least-32-bytes-long-1234")
	otherKey = []byte("another-key-at-least-32-bytes-long-99")
)

func baseClaims() jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer:    "whirl",
		Subject:   "7",
		Audience:  jwt.ClaimStrings{"whirl-api"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
}

func sign(t *testing.T, key any, method jwt.SigningMethod, claims jwt.RegisteredClaims) string {
	t.Helper()

	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)

	return token
}

func Test_JWT_RoundTrip(t *testing.T) {
	token, err := util.GenerateJWT(context.Background(), testKey, 7)
	require.NoError(t, err)

	parsed, err := util.ParseJWT(context.Background(), testKey, token)
	require.NoError(t, err)
	assert.True(t, parsed.Valid)

	sub, err := parsed.Claims.GetSubject()
	require.NoError(t, err)
	assert.Equal(t, "7", sub)
}

func Test_JWT_TTLIs24Hours(t *testing.T) {
	token, err := util.GenerateJWT(context.Background(), testKey, 7)
	require.NoError(t, err)

	parsed, err := util.ParseJWT(context.Background(), testKey, token)
	require.NoError(t, err)

	exp, err := parsed.Claims.GetExpirationTime()
	require.NoError(t, err)

	iat, err := parsed.Claims.GetIssuedAt()
	require.NoError(t, err)

	assert.InDelta(t, util.TokenTTL.Seconds(), exp.Sub(iat.Time).Seconds(), 5)
}

func Test_JWT_EmptyKeyReturnsErrorInsteadOfPanicking(t *testing.T) {
	assert.NotPanics(t, func() {
		_, err := util.GenerateJWT(context.Background(), nil, 7)
		assert.ErrorIs(t, err, util.ErrMissingJWTKey)

		_, err = util.ParseJWT(context.Background(), nil, "some-token")
		assert.ErrorIs(t, err, util.ErrMissingJWTKey)
	})
}

func Test_JWT_RejectsExpiredToken(t *testing.T) {
	claims := baseClaims()
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))

	_, err := util.ParseJWT(context.Background(), testKey, sign(t, testKey, jwt.SigningMethodHS256, claims))
	assert.ErrorIs(t, err, jwt.ErrTokenExpired)
}

func Test_JWT_RejectsWrongAlgorithm(t *testing.T) {
	hs512 := sign(t, testKey, jwt.SigningMethodHS512, baseClaims())

	_, err := util.ParseJWT(context.Background(), testKey, hs512)
	assert.Error(t, err, "HS512 must not be accepted when HS256 is expected")

	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, baseClaims()).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = util.ParseJWT(context.Background(), testKey, none)
	assert.Error(t, err, "alg=none must never be accepted")
}

func Test_JWT_RejectsWrongIssuer(t *testing.T) {
	claims := baseClaims()
	claims.Issuer = "some-other-service"

	_, err := util.ParseJWT(context.Background(), testKey, sign(t, testKey, jwt.SigningMethodHS256, claims))
	assert.ErrorIs(t, err, jwt.ErrTokenInvalidIssuer)
}

func Test_JWT_RejectsWrongAudience(t *testing.T) {
	claims := baseClaims()
	claims.Audience = jwt.ClaimStrings{"some-other-service"}

	_, err := util.ParseJWT(context.Background(), testKey, sign(t, testKey, jwt.SigningMethodHS256, claims))
	assert.ErrorIs(t, err, jwt.ErrTokenInvalidAudience)
}

func Test_JWT_RejectsWrongKey(t *testing.T) {
	token, err := util.GenerateJWT(context.Background(), otherKey, 7)
	require.NoError(t, err)

	_, err = util.ParseJWT(context.Background(), testKey, token)
	assert.Error(t, err)
}

func Test_JWT_RejectsTamperedSignature(t *testing.T) {
	token, err := util.GenerateJWT(context.Background(), testKey, 7)
	require.NoError(t, err)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	sig := []byte(parts[2])
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}

	tampered := parts[0] + "." + parts[1] + "." + string(sig)

	_, err = util.ParseJWT(context.Background(), testKey, tampered)
	assert.Error(t, err)
}
