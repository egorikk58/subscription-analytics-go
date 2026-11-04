package app

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func TestFrontendServesBuiltReactBundle(t *testing.T) {
	index, err := assets.ReadFile("web/dist/index.html")
	handler := frontend()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		if response.Code != 503 || !bytes.Contains(response.Body.Bytes(), []byte(`"data_as_of":null`)) {
			t.Fatalf("missing build must return explicit 503: %d %s", response.Code, response.Body.String())
		}
		return
	}
	if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), index) {
		t.Fatal("Go is not serving the React build")
	}
	script := regexp.MustCompile(`src="(/assets/[^\"]+\.js)"`).FindSubmatch(index)
	if len(script) != 2 {
		t.Fatal("React module entrypoint missing")
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, string(script[1]), nil))
	if response.Code != 200 || bytes.HasPrefix(bytes.TrimSpace(response.Body.Bytes()), []byte("<!doctype")) {
		t.Fatal("JavaScript asset is missing or HTML was served instead")
	}
}
