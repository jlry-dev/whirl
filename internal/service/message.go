package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/repository"
)

type MesssageService interface {
	StoreMessage(ctx context.Context, sender int, receiver int, content string) error
	RetreiveMessages(ctx context.Context, participantOne, participantTwo int) (*dto.MessagesDTO, error)
}

type MessageSrv struct {
	logger  *slog.Logger
	msgRepo repository.MessageRepository
	db      *pgxpool.Pool
}

func (srv *MessageSrv) StoreMessage(ctx context.Context, senderID int, receiverID int, content string) error {
	m := &model.Message{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Content:    content,
		Timestamp:  time.Now(),
	}

	err := srv.msgRepo.CreateMessage(ctx, srv.db, m)
	if err != nil {
		return fmt.Errorf("service: error storing message : %w", err)
	}

	return nil
}

func (srv *MessageSrv) RetreiveMessages(ctx context.Context, participantOne, participantTwo int) (*dto.MessagesDTO, error) {
	messages, err := srv.msgRepo.GetMessages(ctx, srv.db, participantOne, participantTwo)
	if err != nil {
		return nil, fmt.Errorf("service: failed to retrieve messages : %w", err)
	}

	dto := &dto.MessagesDTO{
		Messages: messages,
	}

	return dto, nil
}
