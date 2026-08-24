package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"paperfit-release/internal/domain"
)

type errorResponse struct {
	Error struct {
		Code           string `json:"code"`
		Message        string `json:"message"`
		CurrentVersion int    `json:"currentVersion,omitempty"`
		RequestID      string `json:"requestID,omitempty"`
		Details        any    `json:"details,omitempty"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, err error, currentVersion int) {
	code := domain.ErrorCode(err)
	status := http.StatusBadRequest
	switch code {
	case "not_found":
		status = http.StatusNotFound
	case "unauthorized":
		status = http.StatusUnauthorized
	case "forbidden":
		status = http.StatusForbidden
	case "version_conflict", "idempotency_conflict", "duplicate_case_number", "duplicate_credential", "plan_preview_stale", "review_round_conflict":
		status = http.StatusConflict
	case "internal_error":
		status = http.StatusInternalServerError
	}
	var message = "内部错误"
	response := errorResponse{}
	var domainErr *domain.Error
	if errors.As(err, &domainErr) {
		message = domainErr.Message
		response.Error.Details = domainErr.Details
	}
	response.Error.Code = code
	response.Error.Message = message
	response.Error.CurrentVersion = currentVersion
	response.Error.RequestID = requestIDFrom(r)
	writeJSON(w, status, response)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.NewError("invalid_json", "JSON 请求无效: %v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return domain.NewError("invalid_json", "请求体只能包含一个 JSON 对象")
	}
	return nil
}
