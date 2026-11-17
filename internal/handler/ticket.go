package handler

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// TicketTTL is how long a ticket stays valid. 30s is enough for the handshake,
// so kung ma-log ang URL wala na dayon gamit.
const TicketTTL = 30 * time.Second

type ticketEntry struct {
	userID    int
	expiresAt time.Time
}

// TicketStore gives out short-lived single-use tickets para sa WebSocket.
//
// Dili man kabutang ug Authorization header ang browser sa handshake, so
// normally ibutang ang JWT sa URL. Pero mahabilin na sa logs, tracing, ug
// browser tooling. Ticket is random, opaque, single-use, and expires in
// seconds, so wala nay gamit ang recorded URL.
//
// In memory ra ni, so single instance ra ang supported. Kung mag scale
// horizontally, kinahanglan shared store (like Redis) para ang ticket nga
// gi-issue sa isa ka instance ma-consume sa lain.
type TicketStore struct {
	mu      sync.Mutex
	tickets map[string]ticketEntry
	ttl     time.Duration
	now     func() time.Time
}

func NewTicketStore(ttl time.Duration) *TicketStore {
	return &TicketStore{
		tickets: make(map[string]ticketEntry),
		ttl:     ttl,
		now:     time.Now,
	}
}

// Issue returns a random ticket for userID.
func (s *TicketStore) Issue(userID int) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	value := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.pruneLocked(now)
	s.tickets[value] = ticketEntry{userID: userID, expiresAt: now.Add(s.ttl)}

	return value, nil
}

// Consume returns the userID and burns the ticket, so dili na ma replay.
func (s *TicketStore) Consume(value string) (int, bool) {
	if value == "" {
		return 0, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.tickets[value]
	if !ok {
		return 0, false
	}

	delete(s.tickets, value)

	if !s.now().Before(entry.expiresAt) {
		return 0, false
	}

	return entry.userID, true
}

// pruneLocked removes expired tickets. Gitawag ni sa Issue para dili mo dako
// ang map forever, instead of a background goroutine.
func (s *TicketStore) pruneLocked(now time.Time) {
	for value, entry := range s.tickets {
		if !now.Before(entry.expiresAt) {
			delete(s.tickets, value)
		}
	}
}
