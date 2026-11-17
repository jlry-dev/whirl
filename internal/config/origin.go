package config

import (
	"log/slog"
	"os"
	"strings"
)

// OriginPolicy holds the origins nga pwede mo call sa API.
//
// If empty ang allowlist (walay FRONTEND_ADDRESS), reject tanan request nga
// naay Origin header. Mas maayo mag fail closed kaysa mo allow ug tanan.
//
// Requests nga walay Origin header kay allowed. Dili man na browser, and
// pwede man i-forge ang header sa programmatic client, so browsers ra jud
// ang ma-constrain ani.
type OriginPolicy struct {
	allowed map[string]struct{}
}

// LoadOriginPolicy reads FRONTEND_ADDRESS (comma separated list of origins,
// for example "https://whirl.example,http://localhost:3000").
func LoadOriginPolicy(logger *slog.Logger) *OriginPolicy {
	policy := newOriginPolicy(os.Getenv("FRONTEND_ADDRESS"))

	if len(policy.allowed) == 0 && logger != nil {
		logger.Warn("FRONTEND_ADDRESS is empty: every request carrying an Origin header will be rejected")
	}

	return policy
}

func newOriginPolicy(raw string) *OriginPolicy {
	allowed := make(map[string]struct{})

	for _, part := range strings.Split(raw, ",") {
		if origin := normalizeOrigin(part); origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return &OriginPolicy{allowed: allowed}
}

// normalizeOrigin trims spaces and trailing slashes. Wala man path ang Origin
// header, so "https://example.com/" and "https://example.com" dapat pareho.
// Exact match lang jud, walay prefix/suffix, so "https://example.com.evil.test"
// dili jud mo match sa "https://example.com".
func normalizeOrigin(origin string) string {
	return strings.TrimRight(strings.TrimSpace(origin), "/")
}

// Allows checks if this origin can talk to us.
//
//   - ""      walay Origin header, so dili browser. Allow.
//   - "null"  opaque origin sa sandboxed iframe or file://. Reject always.
//   - other   exact match sa allowlist.
func (p *OriginPolicy) Allows(origin string) bool {
	if origin == "" {
		return true
	}

	if origin == "null" {
		return false
	}

	_, ok := p.allowed[normalizeOrigin(origin)]

	return ok
}
