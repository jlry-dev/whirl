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

func Test_RemoveFriend(t *testing.T) {
	testCases := []struct {
		name      string
		inp       *dto.FriendshipDTO
		mockSetup func(fr *mocks.MockFriendshipRepo)
		wantErr   bool
		expErr    error
	}{
		{
			name: "valid friendship removal",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("DeleteFriendship", mock.Anything, mock.Anything, mock.MatchedBy(func(f *model.Friendship) bool {
					return f.UID_1 == 1 && f.UID_2 == 2
				})).Return(nil)
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
				fr.On("DeleteFriendship", mock.Anything, mock.Anything, mock.Anything).Return(repository.ErrNoRowsFound)
			},
			wantErr: true,
			expErr:  service.ErrNoFriendshipExist,
		},
		{
			name: "repository error",
			inp: &dto.FriendshipDTO{
				From: 1,
				To:   2,
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("DeleteFriendship", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("database error"))
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
			resp, err := srv.RemoveFriend(context.Background(), tc.inp)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				if tc.expErr != nil {
					assert.ErrorIs(t, err, tc.expErr)
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
			name: "valid status update",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("UpdateFriendshipStatus", mock.Anything, mock.Anything, mock.MatchedBy(func(f *model.Friendship) bool {
					return f.UID_1 == 1 && f.UID_2 == 2 && f.Status == model.FriendshipStatus("accepted")
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "friendship not found",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("UpdateFriendshipStatus", mock.Anything, mock.Anything, mock.Anything).Return(repository.ErrNoRowsFound)
			},
			wantErr: true,
			expErr:  service.ErrNoFriendshipExist,
		},
		{
			name: "repository error",
			inp: &dto.FriendshipDTO{
				From:   1,
				To:     2,
				Status: "accepted",
			},
			mockSetup: func(fr *mocks.MockFriendshipRepo) {
				fr.On("UpdateFriendshipStatus", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("database error"))
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
					assert.ErrorIs(t, err, tc.expErr)
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
