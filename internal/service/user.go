package service

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"os"

	"github.com/ajdnik/imghash"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/repository"
)

var ErrInvalidImgFormat = errors.New("Image format unsupported")

type UserService interface {
	UpdateAvatar(ctx context.Context, data *dto.UpdateAvatarDTO) (*dto.UpdateAvatarSuccessDTO, error)
}

type UserSrv struct {
	logger     *slog.Logger
	userRepo   repository.UserRepository
	avatarRepo repository.AvatarRepository
	pHash      *imghash.PHash
	db         *pgxpool.Pool
	cld        *cloudinary.Cloudinary
}

func NewUserService(logger *slog.Logger, userRepo repository.UserRepository, avatarRepo repository.AvatarRepository, db *pgxpool.Pool) UserService {
	cld, err := cloudinary.NewFromURL(os.Getenv("CLOUDINARY_URL"))
	if err != nil {
		panic("initilizing cloudinary SDK failed")
	}

	pHash := imghash.NewPHash()

	return &UserSrv{
		logger:     logger,
		userRepo:   userRepo,
		avatarRepo: avatarRepo,
		pHash:      &pHash,
		db:         db,
		cld:        cld,
	}
}

func (srv *UserSrv) UpdateAvatar(ctx context.Context, data *dto.UpdateAvatarDTO) (*dto.UpdateAvatarSuccessDTO, error) {
	// E decode if image ba jud ang imgData, will return error if not (dili supported ang format)
	img, _, err := image.Decode(data.ImgFile)
	if err != nil {
		if errors.Is(err, image.ErrFormat) {
			return &dto.UpdateAvatarSuccessDTO{}, ErrInvalidImgFormat
		}

		return &dto.UpdateAvatarSuccessDTO{}, err
	}

	pHash := srv.pHash.Calculate(img).String() // the image hash, gamiton para as id to identify duplicated images

	// Check if naa ba sa database base sa pHash
	avatarData, err := srv.avatarRepo.GetAvatarByPhash(ctx, srv.db, pHash)
	if err != nil {
		// Upload the avatar if not exist
		if errors.Is(err, repository.ErrAvatarNotExist) {

			// Used for the public ID
			pid := uuid.New().String()

			cldRsp, err := srv.cld.Upload.Upload(ctx, img, uploader.UploadParams{
				PublicID: pid,
				Folder:   "whirl-avatars",
			})
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: failed to upload img to cloudinary : %w", err)
			}

			// begin transaction
			tx, _ := srv.db.Begin(ctx)
			defer func() {
				_ = tx.Rollback(context.Background())
			}()

			avatar := &model.Avatar{
				PHash:    pHash,
				PublicID: cldRsp.PublicID,
				AssetID:  cldRsp.AssetID,
				URL:      cldRsp.URL,
			}

			// Insert avatar info to db
			avatarData, err := srv.avatarRepo.CreateAvatar(ctx, tx, avatar)
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: failed to update user avatar : %w", err)
			}

			user := &model.User{
				ID:       data.UserID,
				AvatarID: avatarData.ID,
			}

			err = srv.userRepo.UpdateAvatar(ctx, tx, user)
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: failed to update user avatar : %w", err)
			}

			err = tx.Commit(ctx)
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: faield to update user avatar : %w", err)
			}

			return &dto.UpdateAvatarSuccessDTO{
				AvatarURL: avatarData.URL,
			}, nil

		}

		return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: error updating user avatar: %w", err)
	}

	user := &model.User{
		ID:       data.UserID,
		AvatarID: avatarData.ID,
	}

	err = srv.userRepo.UpdateAvatar(ctx, srv.db, user)
	if err != nil {
		return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("serivce: error updating user avatar: %w", err)
	}

	return &dto.UpdateAvatarSuccessDTO{
		AvatarURL: avatarData.URL,
	}, nil
}
