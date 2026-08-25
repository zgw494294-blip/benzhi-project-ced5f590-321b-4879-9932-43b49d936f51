package web

import (
	"io/fs"
	"net/http"
)

func serveAsset(response http.ResponseWriter, name, contentType string) {
	data, err := fs.ReadFile(assets, "assets/"+name)
	if err != nil {
		http.Error(response, "资源不存在", http.StatusNotFound)
		return
	}
	response.Header().Set("Content-Type", contentType)
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(data)
}

func (s *Server) RootHandler(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	http.Redirect(response, request, "/workspace", http.StatusTemporaryRedirect)
}

func (s *Server) WorkspaceHandler(response http.ResponseWriter, _ *http.Request) {
	serveAsset(response, "workspace.html", "text/html; charset=utf-8")
}
func (s *Server) StyleHandler(response http.ResponseWriter, _ *http.Request) {
	serveAsset(response, "style.css", "text/css; charset=utf-8")
}
func (s *Server) ScriptHandler(response http.ResponseWriter, _ *http.Request) {
	serveAsset(response, "app.js", "text/javascript; charset=utf-8")
}

func (s *Server) HealthHandler(response http.ResponseWriter, request *http.Request) {
	status, err := s.service.Integrity(request.Context())
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"status": "ok", "database": status})
}
