package domain

import "fmt"

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func NewDetailedError(code, message string, details any) error {
	return &Error{Code: code, Message: message, Details: details}
}

func (e *Error) Error() string { return e.Message }

func NewError(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ErrorCode(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return "internal_error"
}

func requireText(value, field string) error {
	if value == "" {
		return NewError("validation_error", "%s 不能为空", field)
	}
	return nil
}
