package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/jlry-dev/whirl/internal/util"
)

var (
	ErrNoFriendshipExist = errors.New("service: no friendship exist")
	ErrFriendshipExists  = errors.New("service: friendship already exists")
	ErrMessagingBlocked  = errors.New("service: messaging is blocked")
	ErrNotFriends        = errors.New("service: users are not friends")
	ErrBlockedByOther    = errors.New("service: block was created by the other user")
)

type FriendshipService interface {
	AddFriend(context.Context, *dto.FriendshipDTO) error
	RemoveFriend(context.Context, *dto.FriendshipDTO) (*dto.FrienshipServiceSuccessDTO, error)
	UpdateFriendshipStatus(context.Context, *dto.FriendshipDTO) (*dto.FrienshipServiceSuccessDTO, error)
	RetrieveFriends(ctx context.Context, userID, page int) (*dto.FriendsDetailsResponse, error)
	CheckStatus(context.Context, *dto.FriendshipDTO) (bool, error)
	CanMessage(ctx context.Context, from, to int) error
}

func NewFriendshipService(validate validator.Validate, logger *slog.Logger, frRepo repository.FriendshipRepository, userRepo *repository.UserRepository, db *pgxpool.Pool) FriendshipService {
	return &FriendshipSrv{
		validate: validate,
		logger:   logger,
		frRepo:   frRepo,
		userRepo: *userRepo,
		db:       db,
	}
}

type FriendshipSrv struct {
	validate validator.Validate
	logger   *slog.Logger
	frRepo   repository.FriendshipRepository
	userRepo repository.UserRepository
	db       *pgxpool.Pool
}

func (srv *FriendshipSrv) AddFriend(ctx context.Context, data *dto.FriendshipDTO) error {
	fr := &model.Friendship{
		UID_1: data.From,
		UID_2: data.To,
	}

	tx, err := srv.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("service: failed to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	// Check if users exists
	exists, err := srv.userRepo.CheckUsers(ctx, tx, fr.UID_1, fr.UID_2)
	if err != nil {
		return fmt.Errorf("service: failed to create friendship: %w", err)
	}

	if !exists {
		return fmt.Errorf("service: user participant does not exist")
	}

	err = srv.frRepo.CreateFriendship(ctx, tx, fr)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateFriendship) {
			return ErrFriendshipExists
		}

		return fmt.Errorf("service: failed to create friendship: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("service: failed to commit friendship: %w", err)
	}

	return nil
}

func (srv *FriendshipSrv) RemoveFriend(ctx context.Context, data *dto.FriendshipDTO) (*dto.FrienshipServiceSuccessDTO, error) {
	if err := srv.validate.Struct(data); err != nil {
		vldErrs := err.(validator.ValidationErrors)
		ve := ErrVldFailed{
			Fields: make(map[string]string),
		} // the error struct the holds a map of the field name to the validation message

		for _, e := range vldErrs {
			ve.Fields[e.Field()] = util.GetValidationMessage(e)
		}

		return nil, &ve
	}

	fr := &model.Friendship{
		UID_1: data.From,
		UID_2: data.To,
	}

	current, err := srv.frRepo.GetFriendship(ctx, srv.db, data.From, data.To)
	if err != nil {
		if errors.Is(err, repository.ErrNoRowsFound) {
			return nil, ErrNoFriendshipExist
		}

		return nil, fmt.Errorf("service: failed to load friendship: %w", err)
	}

	// Ang nag-block ra ang pwede mo delete sa blocked row. Otherwise ma-erase
	// ang block and pwede na sad sila mag friend balik.
	if current.Status == model.FriendshipBlocked && (current.BlockedBy == nil || *current.BlockedBy != data.From) {
		return nil, ErrBlockedByOther
	}

	err = srv.frRepo.DeleteFriendship(ctx, srv.db, fr, data.From)
	if err != nil {
		if errors.Is(err, repository.ErrNoRowsFound) {
			return nil, ErrNoFriendshipExist
		}

		return nil, fmt.Errorf("service: failed to delete friendship: %w", err)
	}

	return &dto.FrienshipServiceSuccessDTO{
		Message: "Successfully removed friend",
	}, nil
}

func (srv *FriendshipSrv) UpdateFriendshipStatus(ctx context.Context, data *dto.FriendshipDTO) (*dto.FrienshipServiceSuccessDTO, error) {
	if err := srv.validate.Struct(data); err != nil {
		vldErrs := err.(validator.ValidationErrors)
		ve := ErrVldFailed{
			Fields: make(map[string]string),
		} // the error struct the holds a map of the field name to the validation message

		for _, e := range vldErrs {
			ve.Fields[e.Field()] = util.GetValidationMessage(e)
		}

		return nil, &ve
	}

	status := model.FriendshipStatus(data.Status)
	if status != model.FriendshipAccepted && status != model.FriendshipBlocked {
		return nil, &ErrVldFailed{Fields: map[string]string{"status": "status must be accepted or blocked"}}
	}

	current, err := srv.frRepo.GetFriendship(ctx, srv.db, data.From, data.To)
	if err != nil {
		if errors.Is(err, repository.ErrNoRowsFound) {
			return nil, ErrNoFriendshipExist
		}

		return nil, fmt.Errorf("service: failed to load friendship: %w", err)
	}

	success := &dto.FrienshipServiceSuccessDTO{
		Message: "Successfully updated friendship status",
	}

	switch status {
	case model.FriendshipBlocked:
		// Idempotent ra. Kung blocked na, keep ang original blocker.
		if current.Status == model.FriendshipBlocked {
			return success, nil
		}
	case model.FriendshipAccepted:
		// Wala tay buhaton kung accepted na.
		if current.Status != model.FriendshipBlocked {
			return success, nil
		}

		// Ang nag-block ra ang pwede mo unblock.
		if current.BlockedBy == nil || *current.BlockedBy != data.From {
			return nil, ErrBlockedByOther
		}
	}

	fr := &model.Friendship{
		UID_1:  data.From,
		UID_2:  data.To,
		Status: status,
	}

	err = srv.frRepo.UpdateFriendshipStatus(ctx, srv.db, fr, data.From)
	if err != nil {
		if errors.Is(err, repository.ErrNoRowsFound) {
			// Naunhan mi ug concurrent update, so re-read and decide.
			latest, readErr := srv.frRepo.GetFriendship(ctx, srv.db, data.From, data.To)
			if readErr != nil {
				if errors.Is(readErr, repository.ErrNoRowsFound) {
					return nil, ErrNoFriendshipExist
				}

				return nil, fmt.Errorf("service: failed to re-read friendship: %w", readErr)
			}

			if latest.Status == status {
				return success, nil
			}

			return nil, ErrBlockedByOther
		}

		return nil, fmt.Errorf("service: failed to update friendship status: %w", err)
	}

	return success, nil
}

func (srv *FriendshipSrv) RetrieveFriends(ctx context.Context, userID, page int) (*dto.FriendsDetailsResponse, error) {
	friends, err := srv.frRepo.GetFriends(ctx, srv.db, userID, page)
	if err != nil {
		return nil, fmt.Errorf("service: failed to retrieve friends: %w", err)
	}

	return &dto.FriendsDetailsResponse{
		Friends: friends,
	}, nil
}

/*
This is used to check if two users have relationship record in database

Will return true if user have blocker or accepted
*/
func (srv *FriendshipSrv) CheckStatus(ctx context.Context, data *dto.FriendshipDTO) (bool, error) {
	fr := &model.Friendship{
		UID_1: data.From,
		UID_2: data.To,
	}

	exists, err := srv.frRepo.CheckRelationship(ctx, srv.db, fr)
	if err != nil {
		return false, fmt.Errorf("service: failed to the check friendship : %w", err)
	}

	return exists, nil
}

// CanMessage checks if from is allowed to send a direct message to to.
// Accepted friends lang ang pwede, and blocked always rejects.
func (srv *FriendshipSrv) CanMessage(ctx context.Context, from, to int) error {
	if from == to {
		return ErrNotFriends
	}

	fr, err := srv.frRepo.GetFriendship(ctx, srv.db, from, to)
	if err != nil {
		if errors.Is(err, repository.ErrNoRowsFound) {
			return ErrNotFriends
		}

		return fmt.Errorf("service: failed to check messaging permission : %w", err)
	}

	if fr.Status == model.FriendshipBlocked {
		return ErrMessagingBlocked
	}

	if fr.Status != model.FriendshipAccepted {
		return ErrNotFriends
	}

	return nil
}
