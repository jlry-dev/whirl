package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

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
	LoginHandler(w http.ResponseWriter, r *http.Request)
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
		h.rspHandler.Error(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed), nil)
		return
	}

	typeHeader := strings.Split(r.Header.Get("Content-Type"), ";")
	if typeHeader[0] != "application/json" {
		h.rspHandler.Error(w, http.StatusUnsupportedMediaType, http.StatusText(http.StatusUnsupportedMediaType), nil)
		return
	}

	data := new(dto.RegisterDTO)

	err := json.NewDecoder(r.Body).Decode(data)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	respData, err := h.srv.Register(ctx, data)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		vldErrs, ok := err.(*service.ErrVldFailed)
		if ok {
			// This means that the err is of type ErrVldFailed
			h.rspHandler.Error(w, http.StatusBadRequest, "failed to validate data", vldErrs.Fields)
			return
		}

		if errors.Is(err, service.ErrCountryNotSupported) {
			h.rspHandler.Error(w, http.StatusBadRequest, "country not supported", nil)
			return
		}

		if errors.Is(err, service.ErrUserAlreadyExist) {
			h.rspHandler.Error(w, http.StatusConflict, "user already exist", nil)
			return
		}

		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	respData.Status = http.StatusAccepted
	h.rspHandler.JSON(w, respData.Status, respData)
}

func (h *AuthHandlr) LoginHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	if r.Method != http.MethodPost {
		h.rspHandler.Error(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed), nil)
		return
	}

	typeHeader := strings.Split(r.Header.Get("Content-Type"), ";")
	if typeHeader[0] != "application/json" {
		h.rspHandler.Error(w, http.StatusUnsupportedMediaType, http.StatusText(http.StatusUnsupportedMediaType), nil)
		return
	}

	data := new(dto.LoginDTO)

	if err := json.NewDecoder(r.Body).Decode(data); err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	respData, err := h.srv.Login(ctx, data)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		if errors.Is(err, service.ErrNoUserExist) {
			h.rspHandler.Error(w, http.StatusUnauthorized, "no user found", nil)
			return
		}

		if errors.Is(err, service.ErrInvalidCredential) {
			h.rspHandler.Error(w, http.StatusUnauthorized, "invalid user credentials", nil)
			return
		}

		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	respData.Status = http.StatusOK
	h.rspHandler.JSON(w, respData.Status, respData)
}
