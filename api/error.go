package api

import (
	"fmt"
	"net/http"
)

// HTTPError is an error with a message
type HTTPError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

func httpError(code int, fmtString string, args ...interface{}) *HTTPError {
	return &HTTPError{
		Code:    code,
		Message: fmt.Sprintf(fmtString, args...),
	}
}

func writeError(w http.ResponseWriter, code int, msg string, args ...interface{}) *HTTPError {
	err := httpError(code, msg, args...)
	sendJSON(w, err.Code, err)
	return err
}

func notFoundError(w http.ResponseWriter, msg string, args ...interface{}) *HTTPError {
	return writeError(w, http.StatusNotFound, msg, args...)
}
