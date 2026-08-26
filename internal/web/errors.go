package web

import (
	"errors"
	"net/http"
	"rigging-readiness-desk/internal/domain"
)

type errorBody struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	body := apiError{Code: "INTERNAL_ERROR", Message: "服务暂时无法处理请求"}
	var de *domain.DomainError
	if errors.As(err, &de) {
		body = apiError{Code: string(de.Code), Message: de.Msg, Field: de.Field}
		switch de.Code {
		case domain.ErrValidation:
			status = http.StatusUnprocessableEntity
		case domain.ErrConflict:
			status = http.StatusConflict
		case domain.ErrNotFound:
			status = http.StatusNotFound
		case domain.ErrForbidden:
			status = http.StatusForbidden
		case domain.ErrState:
			status = http.StatusConflict
		}
	}
	writeJSON(w, status, errorBody{Error: body})
}
