package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/service"
)

const MAX_IMAGE_SIZE = 500 * 1024 // 500KB

type UserHandlr struct {
	rsp    *ResponseHandler
	srv    service.UserService
	logger *slog.Logger
}

type UserHandler interface {
	UpdateAvatar(w http.ResponseWriter, r *http.Request)
}

func NewUserHandler(srv service.UserService, logger *slog.Logger) UserHandler {
	rspHandler := NewResponseHandler(logger)

	return &UserHandlr{
		rsp:    rspHandler,
		srv:    srv,
		logger: logger,
	}
}

func (h *UserHandlr) UpdateAvatar(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	if r.Method != http.MethodPost {
		h.rsp.Error(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	// Separate the content type and the content type parameter
	typeHeader := strings.Split(r.Header.Get("Content-Type"), ";")
	if typeHeader[0] != "multipart/form-data" {
		h.rsp.Error(w, http.StatusUnsupportedMediaType, http.StatusText(http.StatusUnsupportedMediaType))
		return
	}

	// Sets the max image size that can be read to 500KB
	r.Body = http.MaxBytesReader(w, r.Body, MAX_IMAGE_SIZE)

	err := r.ParseMultipartForm(MAX_IMAGE_SIZE)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rsp.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	imgFile, _, err := r.FormFile("avatar_img")
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rsp.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	defer imgFile.Close()

	// This requires the authenticator middleware to add the user id to the request context
	userID, ok := ctx.Value("userID").(int)
	if !ok {
		h.logger.Error("handler: failed to get the userID value out of ctx", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rsp.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	dto := dto.UpdateAvatarDTO{
		ImgFile: imgFile,
		UserID:  userID,
	}

	rspData, err := h.srv.UpdateAvatar(ctx, &dto)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		if errors.Is(err, service.ErrInvalidImgFormat) {
			h.rsp.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
			return
		}

		h.rsp.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return

	}

	h.rsp.JSON(w, http.StatusOK, rspData)
}
