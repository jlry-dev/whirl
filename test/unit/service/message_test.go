package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/service"
	"github.com/jlry-dev/whirl/test/mocks"
)

func Test_StoreMessage(t *testing.T) {
	testCases := []struct {
		name       string
		senderID   int
		receiverID int
		content    string
		timestamp  time.Time
		mockSetup  func(mr *mocks.MockMessageRepo)
		wantErr    bool
	}{
		{
			name:       "valid message storage",
			senderID:   1,
			receiverID: 2,
			content:    "Hello, how are you?",
			timestamp:  time.Now(),
			mockSetup: func(mr *mocks.MockMessageRepo) {
				mr.On("CreateMessage", mock.Anything, mock.Anything, mock.MatchedBy(func(m *model.Message) bool {
					return m.SenderID == 1 && m.ReceiverID == 2 && m.Content == "Hello, how are you?"
				})).Return(nil)
			},
			wantErr: false,
		},
		{
			name:       "empty message content",
			senderID:   1,
			receiverID: 2,
			content:    "",
			timestamp:  time.Now(),
			mockSetup: func(mr *mocks.MockMessageRepo) {
				mr.On("CreateMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name:       "repository error",
			senderID:   1,
			receiverID: 2,
			content:    "Test message",
			timestamp:  time.Now(),
			mockSetup: func(mr *mocks.MockMessageRepo) {
				mr.On("CreateMessage", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msgRepo := new(mocks.MockMessageRepo)

			tc.mockSetup(msgRepo)

			srv := service.NewMessageService(nil, msgRepo, nil)
			err := srv.StoreMessage(context.Background(), tc.senderID, tc.receiverID, tc.content, tc.timestamp)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			msgRepo.AssertExpectations(t)
		})
	}
}

func Test_RetrieveMessages(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name           string
		participantOne int
		participantTwo int
		page           int
		mockSetup      func(mr *mocks.MockMessageRepo)
		wantErr        bool
		expCount       int
	}{
		{
			name:           "valid message retrieval",
			participantOne: 1,
			participantTwo: 2,
			page:           1,
			mockSetup: func(mr *mocks.MockMessageRepo) {
				messages := []*model.Message{
					{SenderID: 1, ReceiverID: 2, Content: "Hello", Timestamp: now},
					{SenderID: 2, ReceiverID: 1, Content: "Hi there", Timestamp: now.Add(time.Minute)},
					{SenderID: 1, ReceiverID: 2, Content: "How are you?", Timestamp: now.Add(2 * time.Minute)},
				}
				mr.On("GetMessages", mock.Anything, mock.Anything, 1, 2, 1).Return(messages, nil)
			},
			wantErr:  false,
			expCount: 3,
		},
		{
			name:           "empty message list",
			participantOne: 1,
			participantTwo: 2,
			page:           1,
			mockSetup: func(mr *mocks.MockMessageRepo) {
				mr.On("GetMessages", mock.Anything, mock.Anything, 1, 2, 1).Return([]*model.Message{}, nil)
			},
			wantErr:  false,
			expCount: 0,
		},
		{
			name:           "pagination - page 2",
			participantOne: 1,
			participantTwo: 2,
			page:           2,
			mockSetup: func(mr *mocks.MockMessageRepo) {
				messages := []*model.Message{
					{SenderID: 1, ReceiverID: 2, Content: "Message 1", Timestamp: now},
					{SenderID: 2, ReceiverID: 1, Content: "Message 2", Timestamp: now.Add(time.Minute)},
				}
				mr.On("GetMessages", mock.Anything, mock.Anything, 1, 2, 2).Return(messages, nil)
			},
			wantErr:  false,
			expCount: 2,
		},
		{
			name:           "repository error",
			participantOne: 1,
			participantTwo: 2,
			page:           1,
			mockSetup: func(mr *mocks.MockMessageRepo) {
				mr.On("GetMessages", mock.Anything, mock.Anything, 1, 2, 1).Return(nil, errors.New("database error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msgRepo := new(mocks.MockMessageRepo)

			tc.mockSetup(msgRepo)

			srv := service.NewMessageService(nil, msgRepo, nil)
			resp, err := srv.RetreiveMessages(context.Background(), tc.participantOne, tc.participantTwo, tc.page)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Len(t, resp.Messages, tc.expCount)
			}

			msgRepo.AssertExpectations(t)
		})
	}
}
