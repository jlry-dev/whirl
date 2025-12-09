package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jlry-dev/whirl/internal/service"
)

type MessageHandlr struct {
	rspHandler *ResponseHandler
	srv        service.MessageService
	logger     *slog.Logger
}

type MessageHandler interface {
	RetrieveMessages(w http.ResponseWriter, r *http.Request)
}

func NewMessageHandler(service service.MessageService, rspHandler *ResponseHandler, logger *slog.Logger) MessageHandler {
	return &MessageHandlr{
		srv:        service,
		rspHandler: rspHandler,
		logger:     logger,
	}
}

func (h *MessageHandlr) RetrieveMessages(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()

	if r.Method != http.MethodGet {
		h.logger.Error("retrieve message: invalid http method", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		h.rspHandler.Error(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed), nil)
		return
	}

	// Message participants
	pOne, ok := ctx.Value("userID").(int)
	if !ok {
		h.logger.Error("retrieve message: failed to convert id to int", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		h.rspHandler.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), nil)
		return

	}

	pTwo, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		h.logger.Error("retrieve message: failed to convert id path to int", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		h.rspHandler.Error(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest), nil)
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

	dto, err := h.srv.RetreiveMessages(ctx, pOne, pTwo, page)
	if err != nil {
		h.logger.Error("retrieve message: invalid http method", slog.String("METHOD", r.Method), slog.String("PATH", r.URL.Path))

		h.rspHandler.Error(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError), nil)
		return
	}

	dto.Status = http.StatusOK
	h.rspHandler.JSON(w, http.StatusOK, dto)
}
