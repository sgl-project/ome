package testing

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
)

// TempDir will return a temporary directory and a closer func for deleting
// the directory tree.
func TempDir() (string, func(), error) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return "", nil, err
	}
	return tmp, func() { _ = os.RemoveAll(tmp) }, nil
}

// TempFile will return a temporary file and a closer func for the file.
func TempFile() (*os.File, func(), error) {
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return nil, nil, err
	}
	return tmp, func() { _ = os.Remove(tmp.Name()) }, nil
}

// PerformRequest will make the given request to the supplied handler and return
// an httptest.ResponseRecorder representing the result of making the request.
func PerformRequest(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// PerformSimpleRequest will make the given request to the supplied handler and
// return an httptest.ResponseRecorder indicating the result.
func PerformSimpleRequest(h http.Handler, method, path string) *httptest.ResponseRecorder {
	r, _ := http.NewRequest(method, path, nil)
	return PerformRequest(h, r)
}

// PerformSimpleRequestWithBody is like PerformSimpleRequest, but also takes a
// body.
func PerformSimpleRequestWithBody(h http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	r, _ := http.NewRequest(method, path, body)
	return PerformRequest(h, r)
}
