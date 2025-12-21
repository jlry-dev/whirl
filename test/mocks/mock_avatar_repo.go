package mocks

import (
	"context"

	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/stretchr/testify/mock"
)

type MockAvatarRepo struct {
	mock.Mock
}

func (m *MockAvatarRepo) CreateAvatar(ctx context.Context, qr repository.Queryer, avatar *model.Avatar) (*model.Avatar, error) {
	args := m.Called(ctx, qr, avatar)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*model.Avatar), args.Error(1)
}

func (m *MockAvatarRepo) GetAvatarByPhash(ctx context.Context, qr repository.Queryer, pHash string) (*model.Avatar, error) {
	args := m.Called(ctx, qr, pHash)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*model.Avatar), args.Error(1)
}
