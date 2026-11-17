package handler_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jlry-dev/whirl/internal/handler"
)

func Test_TicketStore_IssuesUniqueTickets(t *testing.T) {
	store := handler.NewTicketStore(time.Minute)

	first, err := store.Issue(1)
	require.NoError(t, err)

	second, err := store.Issue(1)
	require.NoError(t, err)

	assert.NotEmpty(t, first)
	assert.NotEmpty(t, second)
	assert.NotEqual(t, first, second)
}

func Test_TicketStore_ConsumeReturnsUserAndBurnsTicket(t *testing.T) {
	store := handler.NewTicketStore(time.Minute)

	ticket, err := store.Issue(42)
	require.NoError(t, err)

	userID, ok := store.Consume(ticket)
	assert.True(t, ok)
	assert.Equal(t, 42, userID)

	_, ok = store.Consume(ticket)
	assert.False(t, ok, "dili na pwede ma-consume twice")
}

func Test_TicketStore_RejectsUnknownAndEmptyTickets(t *testing.T) {
	store := handler.NewTicketStore(time.Minute)

	_, ok := store.Consume("")
	assert.False(t, ok)

	_, ok = store.Consume("not-a-ticket")
	assert.False(t, ok)
}

func Test_TicketStore_RejectsExpiredTicket(t *testing.T) {
	store := handler.NewTicketStore(10 * time.Millisecond)

	ticket, err := store.Issue(7)
	require.NoError(t, err)

	time.Sleep(25 * time.Millisecond)

	_, ok := store.Consume(ticket)
	assert.False(t, ok, "expired na, so dili na mo authenticate")
}
