package mocks

import (
	"context"

	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/stretchr/testify/mock"
)

type MockMessageRepo struct {
	mock.Mock
}

func (m *MockMessageRepo) CreateMessage(ctx context.Context, qr repository.Queryer, msg *model.Message) error {
	args := m.Called(ctx, qr, msg)
	return args.Error(0)
}

func (m *MockMessageRepo) GetMessages(ctx context.Context, qr repository.Queryer, uidOne, uidTwo, page int) ([]*model.Message, error) {
	args := m.Called(ctx, qr, uidOne, uidTwo, page)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]*model.Message), args.Error(1)
}
