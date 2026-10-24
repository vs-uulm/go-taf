package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestFrontendHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ui/*filepath", frontendHandler(fstest.MapFS{
		"index.html":    {Data: []byte("<html>index</html>")},
		"favicon.ico":   {Data: []byte("icon")},
		"assets/app.js": {Data: []byte("console.log('app')")},
	}))

	tests := []struct {
		path   string
		status int
		body   string
	}{
		{"/ui/", http.StatusOK, "index"},
		{"/ui/tmis/", http.StatusOK, "index"},
		{"/ui/tmis/client/session/template/id/3", http.StatusOK, "index"},
		{"/ui/sessions", http.StatusOK, "index"},
		{"/ui/assets", http.StatusOK, "index"},
		{"/ui/favicon.ico", http.StatusOK, "icon"},
		{"/ui/assets/app.js", http.StatusOK, "console.log"},
		{"/ui/assets/missing.js", http.StatusNotFound, ""},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.body) {
				t.Fatalf("expected %d containing %q, got %d: %q", test.status, test.body, recorder.Code, recorder.Body.String())
			}
		})
	}
}

// TestFrontendFullyEmbedded checks that every file of the built frontend is embedded, as go:embed skips files starting
// with "_" or "." unless the all: prefix is used.
func TestFrontendFullyEmbedded(t *testing.T) {
	err := filepath.WalkDir("frontend/dist", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if _, err := fs.Stat(webFrontend, filepath.ToSlash(path)); err != nil {
			t.Errorf("%s is not embedded: %v", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
