package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jlry-dev/whirl/internal/model/dto"
)

type ResponseHandler struct {
	logger *slog.Logger
}

func NewResponseHandler(logger *slog.Logger) *ResponseHandler {
	return &ResponseHandler{
		logger: logger,
	}
}

func (h *ResponseHandler) JSON(w http.ResponseWriter, statusCode int, payload any) {

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		// In cases where there was encoding errors
		h.logger.Error(fmt.Sprintf("response: failed to encode json response: %v", err.Error()))
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(buf.Bytes())
}

func (h *ResponseHandler) Error(w http.ResponseWriter, statusCode int, errMessage string) {
	payload := dto.JSONError{
		Status: statusCode,
		Error:  errMessage,
	}

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		// In cases where there was encoding errors

		h.logger.Error(fmt.Sprintf("response: failed to encode json response: %v", err.Error()))
		http.Error(w, `{"error": "internal server error"}`, http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(buf.Bytes())
}
