package mocks

import (
	"context"

	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/stretchr/testify/mock"
)

type MockCountryRepo struct {
	mock.Mock
}

func (m *MockCountryRepo) GetIDByISO(ctx context.Context, qr repository.Queryer, iso string) (int, error) {
	args := m.Called(ctx, qr, iso)

	return args.Int(0), args.Error(1)
}
