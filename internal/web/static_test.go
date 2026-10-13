package web

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

// Text assets go out gzipped to a client that accepts it (and decode back to
// the original); a client without gzip, or a binary font, gets the plain file.
func TestGzipAssets(t *testing.T) {
	js := strings.Repeat("console.log('dotin');\n", 500)
	g := newGzipAssets(fstest.MapFS{
		"assets/app.js":  {Data: []byte(js)},
		"assets/f.woff2": {Data: []byte("binary")},
	})
	gin.SetMode(gin.TestMode)
	serve := func(p, accept string) (*httptest.ResponseRecorder, bool) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, p, nil)
		if accept != "" {
			c.Request.Header.Set("Accept-Encoding", accept)
		}
		return rec, g.serve(c, p)
	}

	rec, ok := serve("/assets/app.js", "gzip, deflate, br")
	if !ok || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("js not gzipped: ok=%v headers=%v", ok, rec.Header())
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") &&
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(zr); string(got) != js {
		t.Fatal("gzipped body does not decode to the original asset")
	}
	if rec.Body.Len() >= len(js) {
		t.Fatalf("gzipped %d bytes, original %d", rec.Body.Len(), len(js))
	}

	if _, ok := serve("/assets/app.js", ""); ok {
		t.Fatal("client without gzip must get the plain file")
	}
	if _, ok := serve("/assets/f.woff2", "gzip"); ok {
		t.Fatal("fonts are already compressed and must be served as is")
	}
}
