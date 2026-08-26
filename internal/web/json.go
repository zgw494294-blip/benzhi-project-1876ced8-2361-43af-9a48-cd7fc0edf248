package web

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"rigging-readiness-desk/internal/domain"
)

const maxBodyBytes = int64(1 << 20)

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return domain.NewError(domain.ErrValidation, "Content-Type", "必须为 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return domain.NewError(domain.ErrValidation, "body", "请求 JSON 无效："+err.Error())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return domain.NewError(domain.ErrValidation, "body", "请求体只能包含一个 JSON 对象")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type envelope struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

func dataResponse(w http.ResponseWriter, status int, value any) {
	writeJSON(w, status, envelope{Data: value})
}
