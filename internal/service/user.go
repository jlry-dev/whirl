package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"
	"time"

	"github.com/ajdnik/imghash"
	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jlry-dev/whirl/internal/model"
	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/repository"
)

var ErrUnsupportedImgFormat = errors.New("Image format unsupported")

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
	img, format, err := image.Decode(data.ImgFile)
	if err != nil {
		if errors.Is(err, image.ErrFormat) || format != "jpg" || format != "jpeg" || format != "png" {
			return &dto.UpdateAvatarSuccessDTO{}, ErrUnsupportedImgFormat
		}

		return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("serrvice: avatar update failed to decode image %w", err)
	}

	pHash := srv.pHash.Calculate(img).String() // the image hash, gamiton para as id to identify duplicated images

	// Check if naa ba sa database base sa pHash
	avatarData, err := srv.avatarRepo.GetAvatarByPhash(ctx, srv.db, pHash)
	// Upload the avatar if not exist otherwise user avatarData ditso
	if err != nil {
		if errors.Is(err, repository.ErrAvatarNotExist) {
			var err error
			var imgByte bytes.Buffer

			switch format {
			case "png":
				err = png.Encode(&imgByte, img)
			default: // Cases for jpg or jpeg
				err = jpeg.Encode(&imgByte, img, &jpeg.Options{
					Quality: 90,
				})
			}
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, err
			}

			// Used for the public ID
			pid := uuid.New().String()

			cldRsp, err := srv.cld.Upload.Upload(ctx, &imgByte, uploader.UploadParams{
				PublicID: pid,
				Folder:   "whirl-avatars",
			})
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: failed to upload img to cloudinary : %w", err)
			}

			// The SDK reports Cloudinary error responses as a successful result.
			if cldRsp.Error.Message != "" || cldRsp.PublicID == "" || cldRsp.URL == "" {
				return nil, fmt.Errorf("service: cloudinary rejected upload: %s", cldRsp.Error.Message)
			}

			// The asset is only kept if it ends up referenced by a committed row.
			// Every other path (DB error, rollback, lost dedup race) would leave it
			// orphaned in Cloudinary, so release it here.
			keepAsset := false
			defer func() {
				if keepAsset {
					return
				}

				// The request context may already be cancelled on the error paths.
				dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer cancel()

				if _, derr := srv.cld.Upload.Destroy(dctx, uploader.DestroyParams{PublicID: pid}); derr != nil {
					srv.logger.Error("service: failed to clean up unused avatar upload",
						slog.String("public_id", pid), slog.Any("error", derr))
				}
			}()

			// begin transaction
			tx, err := srv.db.Begin(ctx)
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: failed to begin avatar transaction: %w", err)
			}

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
			createdAvatar, err := srv.avatarRepo.CreateAvatar(ctx, tx, avatar)
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: failed to update user avatar : %w", err)
			}

			user := &model.User{
				ID:       data.UserID,
				AvatarID: createdAvatar.ID,
			}

			err = srv.userRepo.UpdateAvatar(ctx, tx, user)
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: failed to update user avatar : %w", err)
			}

			err = tx.Commit(ctx)
			if err != nil {
				return &dto.UpdateAvatarSuccessDTO{}, fmt.Errorf("service: failed to update user avatar : %w", err)
			}

			// If another request won the dedup race, the committed row points at
			// that request's asset and ours is released by the deferred cleanup.
			keepAsset = createdAvatar.PublicID == pid

			return &dto.UpdateAvatarSuccessDTO{
				AvatarURL: createdAvatar.URL,
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
