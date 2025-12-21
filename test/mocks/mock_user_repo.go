package mocks

import (
	"context"

	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/stretchr/testify/mock"
)

type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) CreateUser(ctx context.Context, qr repository.Queryer, user *model.User) (int, error) {
	args := m.Called(ctx, qr, user)

	return args.Int(0), args.Error(1)
}

func (m *MockUserRepo) UpdateAvatar(ctx context.Context, qr repository.Queryer, user *model.User) error {
	args := m.Called(ctx, qr, user)

	return args.Error(0)
}

func (m *MockUserRepo) GetUserWithCountryByUsername(ctx context.Context, qr repository.Queryer, username string) (*dto.UserWithCountryDTO, error) {
	args := m.Called(ctx, qr, username)

	// Used to avoid when we input a nil return at index 0
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*dto.UserWithCountryDTO), args.Error(1)
}

func (m *MockUserRepo) CheckUsers(ctx context.Context, qr repository.Queryer, userIDs ...int) (bool, error) {
	args := m.Called(ctx, qr, userIDs)

	return args.Bool(0), args.Error(1)
}
