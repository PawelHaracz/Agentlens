package api_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
)

func stringReader(s string) io.Reader {
	return strings.NewReader(s)
}

func decodeJSON(w *httptest.ResponseRecorder, v interface{}) error {
	return json.Unmarshal(w.Body.Bytes(), v)
}
