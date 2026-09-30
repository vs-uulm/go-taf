package web

import (
	"net/http"
	"net/http/httptest"
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
