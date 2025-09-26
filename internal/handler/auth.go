package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/service"
)

type AuthHandlr struct {
	rspHandler *ResponseHandler
	srv        service.AuthService
	logger     *slog.Logger
}

type AuthHandler interface {
	RegisterHandler(w http.ResponseWriter, r *http.Request)
}

func NewAuthHandler(service service.AuthService, rspHandler *ResponseHandler, logger *slog.Logger) AuthHandler {
	return &AuthHandlr{
		srv:        service,
		rspHandler: rspHandler,
		logger:     logger,
	}
}

func (h *AuthHandlr) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	if r.Method != http.MethodPost {
		h.rspHandler.Error(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		h.rspHandler.Error(w, http.StatusUnsupportedMediaType, http.StatusText(http.StatusUnsupportedMediaType))
		return
	}

	data := new(dto.RegisterDTO)

	err := json.NewDecoder(r.Body).Decode(data)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	respData, err := h.srv.Register(ctx, data)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		if errors.Is(err, service.ErrValidationFailed) || errors.Is(err, service.ErrCountryNotSupported) {
			h.rspHandler.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
			return
		}

		if errors.Is(err, service.ErrUserAlreadyExist) {
			h.rspHandler.Error(w, http.StatusConflict, http.StatusText(http.StatusConflict))
			return
		}

		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	respData.Status = http.StatusAccepted
	h.rspHandler.JSON(w, http.StatusAccepted, respData)
}
