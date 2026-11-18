package mocks

import (
	"context"

	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/stretchr/testify/mock"
)

type MockFriendshipRepo struct {
	mock.Mock
}

func (m *MockFriendshipRepo) CreateFriendship(ctx context.Context, qr repository.Queryer, fr *model.Friendship) error {
	args := m.Called(ctx, qr, fr)
	return args.Error(0)
}

func (m *MockFriendshipRepo) DeleteFriendship(ctx context.Context, qr repository.Queryer, fr *model.Friendship) error {
	args := m.Called(ctx, qr, fr)
	return args.Error(0)
}

func (m *MockFriendshipRepo) UpdateFriendshipStatus(ctx context.Context, qr repository.Queryer, fr *model.Friendship) error {
	args := m.Called(ctx, qr, fr)
	return args.Error(0)
}

func (m *MockFriendshipRepo) GetFriends(ctx context.Context, qr repository.Queryer, userID, page int) ([]*dto.FriendDetails, error) {
	args := m.Called(ctx, qr, userID, page)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]*dto.FriendDetails), args.Error(1)
}

func (m *MockFriendshipRepo) CheckRelationship(ctx context.Context, qr repository.Queryer, fr *model.Friendship) (bool, error) {
	args := m.Called(ctx, qr, fr)
	return args.Bool(0), args.Error(1)
}

func (m *MockFriendshipRepo) GetFriendshipStatus(ctx context.Context, qr repository.Queryer, fr *model.Friendship) (model.FriendshipStatus, bool, error) {
	args := m.Called(ctx, qr, fr)

	// status is a named string type, so handle both forms.
	var status model.FriendshipStatus
	switch v := args.Get(0).(type) {
	case model.FriendshipStatus:
		status = v
	case string:
		status = model.FriendshipStatus(v)
	}

	return status, args.Bool(1), args.Error(2)
}
