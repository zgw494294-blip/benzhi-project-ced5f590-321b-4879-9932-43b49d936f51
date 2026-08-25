package web

import (
	"net/http"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
)

type submitReviewRequest struct {
	application.CommandMeta
	Revision int `json:"revision"`
}

func (s *Server) UpsertMeasureHandler(response http.ResponseWriter, request *http.Request) {
	var command application.UpsertMeasureCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.UpsertMeasure(request.Context(), request.PathValue("id"), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) SubmitReviewHandler(response http.ResponseWriter, request *http.Request) {
	var command submitReviewRequest
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.SubmitReview(request.Context(), request.PathValue("id"), command.Revision, command.CommandMeta)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) ReviewMeasureHandler(response http.ResponseWriter, request *http.Request) {
	var command application.ReviewCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.ReviewMeasure(request.Context(), request.PathValue("id"), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) BatchReviewMeasuresHandler(response http.ResponseWriter, request *http.Request) {
	var command application.BatchReviewCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.BatchReviewMeasures(request.Context(), request.PathValue("id"), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) AddRemediationHandler(response http.ResponseWriter, request *http.Request) {
	var command application.RemediationCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.AddRemediation(request.Context(), request.PathValue("id"), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) VerifyRemediationHandler(response http.ResponseWriter, request *http.Request) {
	var command application.VerificationCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.VerifyRemediation(request.Context(), request.PathValue("id"), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}
