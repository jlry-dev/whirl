package config_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jlry-dev/whirl/internal/config"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func Test_OriginPolicy_AllowsConfiguredOrigins(t *testing.T) {
	t.Setenv("FRONTEND_ADDRESS", "https://whirl.example, http://localhost:3000/")

	policy := config.LoadOriginPolicy(discardLogger())

	assert.True(t, policy.Allows("https://whirl.example"))
	assert.True(t, policy.Allows("http://localhost:3000"), "trailing slash sa config dapat ma-normalize")
	assert.False(t, policy.Allows("https://evil.example"))
}

func Test_OriginPolicy_FailsClosedWhenUnset(t *testing.T) {
	t.Setenv("FRONTEND_ADDRESS", "")

	policy := config.LoadOriginPolicy(discardLogger())

	assert.False(t, policy.Allows("https://whirl.example"), "walay allowlist, so reject tanan browser origin")
	assert.False(t, policy.Allows("http://localhost:3000"))
}

func Test_OriginPolicy_AllowsMissingOrigin(t *testing.T) {
	t.Setenv("FRONTEND_ADDRESS", "https://whirl.example")

	policy := config.LoadOriginPolicy(discardLogger())

	// Non-browser callers walay Origin. Ang policy para ra ni sa cross-site
	// browser requests, dili ni authentication.
	assert.True(t, policy.Allows(""))
}

func Test_OriginPolicy_RejectsNullOrigin(t *testing.T) {
	t.Setenv("FRONTEND_ADDRESS", "https://whirl.example")

	policy := config.LoadOriginPolicy(discardLogger())

	assert.False(t, policy.Allows("null"))
}

func Test_OriginPolicy_MatchesExactly(t *testing.T) {
	t.Setenv("FRONTEND_ADDRESS", "https://whirl.example")

	policy := config.LoadOriginPolicy(discardLogger())

	assert.False(t, policy.Allows("https://whirl.example.evil.test"), "suffix trick, dili jud mo match")
	assert.False(t, policy.Allows("https://evil.test/https://whirl.example"), "prefix trick, dili jud mo match")
	assert.False(t, policy.Allows("http://whirl.example"), "dapat pareho ang scheme")
	assert.False(t, policy.Allows("https://whirl.example:8443"), "dapat pareho ang port")
}
