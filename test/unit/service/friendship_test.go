package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/jlry-dev/whirl/internal/service"
	"github.com/jlry-dev/whirl/test/mocks"
)

// intPtr kay BlockedBy *int.
func intPtr(v int) *int {
	return &v
}

func Test_RemoveFriend(t *testing.T) {
	testCases := []struct {
		name      string
		inp       *dto.FriendshipDTO
		mockSetup func(fr *mocks.MockFriendshipRepo)
		wantErr   bool
		expErr    error
	}{
		{
			name: "accepted friendship removal",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipAccepted}, nil)
				fr.On("DeleteFriendship", mock.Anything, mock.Anything, mock.MatchedBy(func(f *model.Friendship) bool {
					return f.UID_1 == 1 && f.UID_2 == 2
				}), 1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "friendship not found",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(nil, repository.ErrNoRowsFound)
			},
			wantErr: true,
			expErr:  service.ErrNoFriendshipExist,
		},
		{
			name: "blocker can delete the blocked friendship",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(1)}, nil)
				fr.On("DeleteFriendship", mock.Anything, mock.Anything, mock.Anything, 1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "blocked user cannot delete the block",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(2)}, nil)
				// No DeleteFriendship expectation: if the service calls it, the
				// mock fails loudly.
			},
			wantErr: true,
			expErr:  service.ErrBlockedByOther,
		},
		{
			name: "legacy blocked row without blocker cannot be deleted",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked}, nil)
			},
			wantErr: true,
			expErr:  service.ErrBlockedByOther,
		},
		{
			name: "load repository error",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
		{
			name: "delete repository error",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipAccepted}, nil)
				fr.On("DeleteFriendship", mock.Anything, mock.Anything, mock.Anything, 1).
					Return(errors.New("database error"))
			},
			wantErr: true,
		},
		{
			name: "delete lost the row to a concurrent update",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipAccepted}, nil)
				fr.On("DeleteFriendship", mock.Anything, mock.Anything, mock.Anything, 1).
					Return(repository.ErrNoRowsFound)
			},
			wantErr: true,
			expErr:  service.ErrNoFriendshipExist,
		},
		{
			name: "validation failure",
			inp: &dto.FriendshipDTO{
				From: 1,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {},
			wantErr:   true,
			expErr:    &service.ErrVldFailed{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vld := validator.New(validator.WithRequiredStructEnabled())
			frRepo := new(mocks.MockFriendshipRepo)
			userRepo := new(mocks.MockUserRepo)

			tc.mockSetup(frRepo)

			var userRepoInterface repository.UserRepository = userRepo
			srv := service.NewFriendshipService(*vld, nil, frRepo, &userRepoInterface, nil)
			resp, err := srv.RemoveFriend(context.Background(), tc.inp)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				if tc.expErr != nil {
					ErrorTestHelper(t, err, tc.expErr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "Successfully removed friend", resp.Message)
			}

			frRepo.AssertExpectations(t)
		})
	}
}

func Test_UpdateFriendshipStatus(t *testing.T) {
	testCases := []struct {
		name      string
		inp       *dto.FriendshipDTO
		mockSetup func(fr *mocks.MockFriendshipRepo)
		wantErr   bool
		expErr    error
	}{
		{
			name: "accept an already accepted friendship is a no-op",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipAccepted}, nil)
				// No UpdateFriendshipStatus expectation on purpose.
			},
			wantErr: false,
		},
		{
			name: "caller can block an accepted friendship",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "blocked",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipAccepted}, nil)
				fr.On("UpdateFriendshipStatus", mock.Anything, mock.Anything, mock.MatchedBy(func(f *model.Friendship) bool {
					return f.UID_1 == 1 && f.UID_2 == 2 && f.Status == model.FriendshipBlocked
				}), 1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "re-blocking keeps the original blocker",
			inp: &dto.FriendshipDTO{
				From:   2,
				To:     1,
				Status: "blocked",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 2, 1).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(1)}, nil)
				// First block wins, so no update must happen.
			},
			wantErr: false,
		},
		{
			name: "blocker can unblock",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(1)}, nil)
				fr.On("UpdateFriendshipStatus", mock.Anything, mock.Anything, mock.MatchedBy(func(f *model.Friendship) bool {
					return f.UID_1 == 1 && f.UID_2 == 2 && f.Status == model.FriendshipAccepted
				}), 1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "blocked user cannot unblock",
			inp: &dto.FriendshipDTO{
				From:   2,
				To:     1,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 2, 1).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(1)}, nil)
			},
			wantErr: true,
			expErr:  service.ErrBlockedByOther,
		},
		{
			name: "legacy blocked row without blocker cannot be unblocked",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked}, nil)
			},
			wantErr: true,
			expErr:  service.ErrBlockedByOther,
		},
		{
			name: "friendship not found",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(nil, repository.ErrNoRowsFound)
			},
			wantErr: true,
			expErr:  service.ErrNoFriendshipExist,
		},
		{
			name: "unknown status is rejected",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "pending",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {},
			wantErr:   true,
			expErr:    &service.ErrVldFailed{},
		},
		{
			name: "missing status is rejected instead of hitting the enum",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {},
			wantErr:   true,
			expErr:    &service.ErrVldFailed{},
		},
		{
			name: "concurrent update to the same status is a success",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(1)}, nil).Once()
				fr.On("UpdateFriendshipStatus", mock.Anything, mock.Anything, mock.Anything, 1).
					Return(repository.ErrNoRowsFound).Once()
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipAccepted}, nil).Once()
			},
			wantErr: false,
		},
		{
			name: "concurrent update blocked by the other user is rejected",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(1)}, nil).Once()
				fr.On("UpdateFriendshipStatus", mock.Anything, mock.Anything, mock.Anything, 1).
					Return(repository.ErrNoRowsFound).Once()
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(2)}, nil).Once()
			},
			wantErr: true,
			expErr:  service.ErrBlockedByOther,
		},
		{
			name: "concurrent delete is reported as not found",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(1)}, nil).Once()
				fr.On("UpdateFriendshipStatus", mock.Anything, mock.Anything, mock.Anything, 1).
					Return(repository.ErrNoRowsFound).Once()
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(nil, repository.ErrNoRowsFound).Once()
			},
			wantErr: true,
			expErr:  service.ErrNoFriendshipExist,
		},
		{
			name: "load repository error",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
		{
			name: "update repository error",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "blocked",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipAccepted}, nil)
				fr.On("UpdateFriendshipStatus", mock.Anything, mock.Anything, mock.Anything, 1).
					Return(errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vld := validator.New(validator.WithRequiredStructEnabled())
			frRepo := new(mocks.MockFriendshipRepo)
			userRepo := new(mocks.MockUserRepo)

			tc.mockSetup(frRepo)

			var userRepoInterface repository.UserRepository = userRepo
			srv := service.NewFriendshipService(*vld, nil, frRepo, &userRepoInterface, nil)
			resp, err := srv.UpdateFriendshipStatus(context.Background(), tc.inp)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				if tc.expErr != nil {
					ErrorTestHelper(t, err, tc.expErr)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "Successfully updated friendship status", resp.Message)
			}

			frRepo.AssertExpectations(t)
		})
	}
}

func Test_RetrieveFriends(t *testing.T) {
	testCases := []struct {
		name      string
		userID    int
		page      int
		mockSetup func(fr *mocks.MockFriendshipRepo)
		wantErr   bool
		expCount  int
	}{
		{
			name:   "valid retrieval",
			userID: 1,
			page:   1,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				friends := []*dto.FriendDetails{
					{ID: 2, Username: "user2"},
					{ID: 3, Username: "user3"},
				}
				fr.On("GetFriends", mock.Anything, mock.Anything, 1, 1).Return(friends, nil)
			},
			wantErr:  false,
			expCount: 2,
		},
		{
			name:   "empty friends list",
			userID: 1,
			page:   1,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriends", mock.Anything, mock.Anything, 1, 1).Return([]*dto.FriendDetails{}, nil)
			},
			wantErr:  false,
			expCount: 0,
		},
		{
			name:   "repository error",
			userID: 1,
			page:   1,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriends", mock.Anything, mock.Anything, 1, 1).Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vld := validator.New(validator.WithRequiredStructEnabled())
			frRepo := new(mocks.MockFriendshipRepo)
			userRepo := new(mocks.MockUserRepo)

			tc.mockSetup(frRepo)

			var userRepoInterface repository.UserRepository = userRepo
			srv := service.NewFriendshipService(*vld, nil, frRepo, &userRepoInterface, nil)
			resp, err := srv.RetrieveFriends(context.Background(), tc.userID, tc.page)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Len(t, resp.Friends, tc.expCount)
			}

			frRepo.AssertExpectations(t)
		})
	}
}

func Test_CheckStatus(t *testing.T) {
	testCases := []struct {
		name      string
		inp       *dto.FriendshipDTO
		mockSetup func(fr *mocks.MockFriendshipRepo)
		wantErr   bool
		expExists bool
	}{
		{
			name: "relationship exists",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("CheckRelationship", mock.Anything, mock.Anything, mock.MatchedBy(func(f *model.Friendship) bool {
					return f.UID_1 == 1 && f.UID_2 == 2
				})).Return(true, nil)
			},
			wantErr:   false,
			expExists: true,
		},
		{
			name: "relationship does not exist",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("CheckRelationship", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
			},
			wantErr:   false,
			expExists: false,
		},
		{
			name: "repository error",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("CheckRelationship", mock.Anything, mock.Anything, mock.Anything).Return(false, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vld := validator.New(validator.WithRequiredStructEnabled())
			frRepo := new(mocks.MockFriendshipRepo)
			userRepo := new(mocks.MockUserRepo)

			tc.mockSetup(frRepo)

			var userRepoInterface repository.UserRepository = userRepo
			srv := service.NewFriendshipService(*vld, nil, frRepo, &userRepoInterface, nil)
			exists, err := srv.CheckStatus(context.Background(), tc.inp)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expExists, exists)
			}

			frRepo.AssertExpectations(t)
		})
	}
}

func Test_CanMessage(t *testing.T) {
	testCases := []struct {
		name      string
		from      int
		to        int
		mockSetup func(fr *mocks.MockFriendshipRepo)
		wantErr   bool
		expErr    error
	}{
		{
			name: "accepted friends can message",
			from: 1,
			to:   2,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipAccepted}, nil)
			},
		},
		{
			name: "blocked pair is rejected",
			from: 1,
			to:   2,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(2)}, nil)
			},
			wantErr: true,
			expErr:  service.ErrMessagingBlocked,
		},
		{
			name: "blocked pair is rejected no matter who asks",
			from: 2,
			to:   1,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 2, 1).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipBlocked, BlockedBy: intPtr(2)}, nil)
			},
			wantErr: true,
			expErr:  service.ErrMessagingBlocked,
		},
		{
			name: "non-friends are rejected",
			from: 1,
			to:   2,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(nil, repository.ErrNoRowsFound)
			},
			wantErr: true,
			expErr:  service.ErrNotFriends,
		},
		{
			name: "pending relationship is rejected",
			from: 1,
			to:   2,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(&model.Friendship{UID_1: 1, UID_2: 2, Status: model.FriendshipStatus("pending")}, nil)
			},
			wantErr: true,
			expErr:  service.ErrNotFriends,
		},
		{
			name:      "self message is rejected without a query",
			from:      1,
			to:        1,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {},
			wantErr:   true,
			expErr:    service.ErrNotFriends,
		},
		{
			name: "repository error is propagated",
			from: 1,
			to:   2,
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("GetFriendship", mock.Anything, mock.Anything, 1, 2).
					Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vld := validator.New(validator.WithRequiredStructEnabled())
			frRepo := new(mocks.MockFriendshipRepo)
			userRepo := new(mocks.MockUserRepo)

			tc.mockSetup(frRepo)

			var userRepoInterface repository.UserRepository = userRepo
			srv := service.NewFriendshipService(*vld, nil, frRepo, &userRepoInterface, nil)
			err := srv.CanMessage(context.Background(), tc.from, tc.to)

			if tc.wantErr {
				assert.Error(t, err)
				if tc.expErr != nil {
					assert.ErrorIs(t, err, tc.expErr)
				}
			} else {
				assert.NoError(t, err)
			}

			frRepo.AssertExpectations(t)
		})
	}
}
