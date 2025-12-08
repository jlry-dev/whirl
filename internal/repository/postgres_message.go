package repository

import (
	"context"
	"fmt"

	"github.com/jlry-dev/whirl/internal/model"
)

type MessageRepo struct{}

func NewMessageRepository() MessageRepository {
	return &MessageRepo{}
}

func (r *MessageRepo) CreateMessage(ctx context.Context, qr Queryer, ch *model.Message) error {
	qry := `INSERT INTO messages (sender_id, receiver_id, content, timestamp) VALUES ($1, $2, $3, $4)`

	if _, err := qr.Exec(ctx, qry, ch.SenderID, ch.ReceiverID, ch.Content, ch.Timestamp); err != nil {
		return fmt.Errorf("repo: failed to create message: %w", err)
	}

	return nil
}

func (r *MessageRepo) GetMessages(ctx context.Context, qr Queryer, uidOne, uidTwo int) ([]*model.Message, error) {
	qry := `SELECT sender_id, receiver_id, content, timestamp FROM messages as m WHERE (m.sender_id = $1 AND m.receiver_id = $2) OR (m.sender_id = $2 AND m.receiver_id = $1)`

	rows, err := qr.Query(ctx, qry, uidOne, uidTwo)
	if err != nil {
		return nil, fmt.Errorf("repo: failed to get messages : %w", err)
	}
	defer rows.Close()

	messages := make([]*model.Message, 0, 100)
	for rows.Next() {
		var m model.Message
		err := rows.Scan(&m.SenderID, &m.ReceiverID, &m.Content, &m.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("repo: failed to scan message row : %w", err)
		}

		messages = append(messages, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repo: error during iteration : %w", err)
	}

	return messages, nil
}
