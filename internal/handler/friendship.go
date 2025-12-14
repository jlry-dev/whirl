package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jlry-dev/whirl/internal/model/dto"
	"github.com/jlry-dev/whirl/internal/service"
)

type FriendshipHandler interface {
	RemoveFriend(w http.ResponseWriter, r *http.Request)
	UpdateFriendshipStatus(w http.ResponseWriter, r *http.Request)
	RetrieveFriends(w http.ResponseWriter, r *http.Request)
}

type FriendshipHandlr struct {
	logger     *slog.Logger
	rspHandler *ResponseHandler
	frSrv      service.FriendshipService
}

func NewFriendshipHandler(logger *slog.Logger, rspHandler *ResponseHandler, frSrv service.FriendshipService) FriendshipHandler {
	return &FriendshipHandlr{
		logger:     logger,
		rspHandler: rspHandler,
		frSrv:      frSrv,
	}
}

func (h *FriendshipHandlr) RemoveFriend(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	if r.Method != http.MethodDelete {
		h.logger.Error("remove friend: invalid http method", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed), nil)
		return
	}

	typeHeader := strings.Split(r.Header.Get("Content-Type"), ";")
	if typeHeader[0] != "application/json" {
		h.logger.Error("remove friend unsupported media format", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusUnsupportedMediaType, http.StatusText(http.StatusUnsupportedMediaType), nil)
		return
	}

	// This requires the authenticator middleware to add the user id to the request context
	userID, ok := ctx.Value("userID").(int)
	if !ok {
		h.logger.Error("handler: failed to get the userID value out of ctx", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	data := new(dto.FriendshipDTO)

	err := json.NewDecoder(r.Body).Decode(data)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), nil)
		return
	}

	data.From = userID

	rspData, err := h.frSrv.RemoveFriend(ctx, data)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		vldErrs, ok := err.(*service.ErrVldFailed)
		if ok {
			// This means that the err is of type ErrVldFailed
			h.rspHandler.Error(w, http.StatusBadRequest, "failed to validate data", vldErrs.Fields)
			return
		}

		if errors.Is(err, service.ErrNoFriendshipExist) {
			h.rspHandler.Error(w, http.StatusNotFound, "no friendship record found", nil)
			return
		}

		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	rspData.Status = http.StatusOK
	h.rspHandler.JSON(w, http.StatusOK, rspData)
}

func (h *FriendshipHandlr) UpdateFriendshipStatus(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	if r.Method != http.MethodPut {
		h.logger.Error("remove friend: invalid http method", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed), nil)
		return
	}

	typeHeader := strings.Split(r.Header.Get("Content-Type"), ";")
	if typeHeader[0] != "application/json" {
		h.logger.Error("remove friend unsupported media format", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusUnsupportedMediaType, http.StatusText(http.StatusUnsupportedMediaType), nil)
		return
	}

	userID, ok := ctx.Value("userID").(int)
	if !ok {
		h.logger.Error("handler: failed to get the userID value out of ctx", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	data := new(dto.FriendshipDTO)

	err := json.NewDecoder(r.Body).Decode(data)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), nil)
		return
	}

	data.From = userID

	rspData, err := h.frSrv.UpdateFriendshipStatus(ctx, data)
	if err != nil {
		h.logger.Error(err.Error(), slog.String("w", r.Method), slog.String("PATH", r.URL.Path))

		vldErrs, ok := err.(*service.ErrVldFailed)
		if ok {
			// This means that the err is of type ErrVldFailed
			h.rspHandler.Error(w, http.StatusBadRequest, "failed to validate data", vldErrs.Fields)
			return
		}

		if errors.Is(err, service.ErrNoFriendshipExist) {
			h.rspHandler.Error(w, http.StatusNotFound, "no friendship record not found", nil)
			return
		}

		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	rspData.Status = http.StatusOK
	h.rspHandler.JSON(w, http.StatusOK, rspData)
}

func (h *FriendshipHandlr) RetrieveFriends(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	if r.Method != http.MethodGet {
		h.logger.Error("remove friend: invalid http method", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed), nil)
		return
	}

	typeHeader := strings.Split(r.Header.Get("Content-Type"), ";")
	if typeHeader[0] != "application/json" {
		h.logger.Error("remove friend unsupported media format", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusUnsupportedMediaType, http.StatusText(http.StatusUnsupportedMediaType), nil)
		return
	}

	userID, ok := ctx.Value("userID").(int)
	if !ok {
		h.logger.Error("handler: failed to get the userID value out of ctx", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))
		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		h.logger.Error("retrieve message: failed to convert page to int", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		h.rspHandler.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), nil)
		return
	}

	if page == 0 {
		// default
		page = 1
	}

	dto, err := h.frSrv.RetrieveFriends(ctx, userID, page)
	if err != nil {
		h.logger.Error("retrieve message: invalid http method", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	dto.Status = http.StatusOK
	h.rspHandler.JSON(w, http.StatusOK, dto)
}
