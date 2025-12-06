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

var ErrNoFriendshipExist = errors.New("service: no friendship exist")

type FriendshipService interface {
	AddFriend(context.Context, *dto.FriendshipDTO) error
	RemoveFriend(context.Context, *dto.FriendshipDTO) (*dto.FrienshipServiceSuccessDTO, error)
	UpdateFriendshipStatus(context.Context, *dto.FriendshipDTO) (*dto.FrienshipServiceSuccessDTO, error)
	CheckStatus(context.Context, *dto.FriendshipDTO) (bool, error)
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

	tx, _ := srv.db.Begin(ctx)
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
		return fmt.Errorf("service: failed to create friendship: %w", err)
	}

	tx.Commit(ctx)

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

	err := srv.frRepo.DeleteFriendship(ctx, srv.db, fr)
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

	fr := &model.Friendship{
		UID_1:  data.From,
		UID_2:  data.To,
		Status: model.FriendshipStatus(data.Status),
	}

	err := srv.frRepo.UpdateFriendshipStatus(ctx, srv.db, fr)
	if err != nil {
		if errors.Is(err, repository.ErrNoRowsFound) {
			return nil, ErrNoFriendshipExist
		}

		return nil, fmt.Errorf("service: failed to delete friendship: %w", err)
	}

	return &dto.FrienshipServiceSuccessDTO{
		Message: "Successfully updated friendship status",
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
