package lib

import (
	"errors"
	"net/http"
	"testing"

	"github.com/sebrandon1/go-toolbox/httpx"
)

func TestStatusCodeErrorMapsToHTTPError(t *testing.T) {
	err := StatusCodeError(http.StatusNotFound)

	var codeErr StatusCodeError
	if !errors.As(err, &codeErr) || int(codeErr) != http.StatusNotFound {
		t.Fatalf("expected StatusCodeError(404), got %v", err)
	}
	if !IsStatusCodeError(err) {
		t.Error("IsStatusCodeError returned false for StatusCodeError")
	}

	var httpErr *httpx.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected errors.As to find *httpx.HTTPError, got %v", err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Errorf("HTTPError.StatusCode = %d, want %d", httpErr.StatusCode, http.StatusNotFound)
	}
	if !httpx.IsHTTPError(err) {
		t.Error("httpx.IsHTTPError returned false for StatusCodeError")
	}
	if got, want := err.Error(), "server returned 404 Not Found"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
