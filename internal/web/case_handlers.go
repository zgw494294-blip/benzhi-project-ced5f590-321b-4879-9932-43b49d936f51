package web

import (
	"net/http"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
)

func (s *Server) ListCasesHandler(response http.ResponseWriter, request *http.Request) {
	items, err := s.service.List(request.Context())
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) CreateCaseHandler(response http.ResponseWriter, request *http.Request) {
	var command application.CreateCaseCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.CreateCase(request.Context(), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, result)
}

func (s *Server) GetCaseHandler(response http.ResponseWriter, request *http.Request) {
	result, err := s.service.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}
