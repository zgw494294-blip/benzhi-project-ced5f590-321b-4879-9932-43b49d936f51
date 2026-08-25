package web

import (
	"net/http"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
)

func (s *Server) AddSurveyHandler(response http.ResponseWriter, request *http.Request) {
	var command application.AddSurveyCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.AddSurvey(request.Context(), request.PathValue("id"), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) CorrectSurveyHandler(response http.ResponseWriter, request *http.Request) {
	var command application.CorrectSurveyCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.CorrectSurvey(request.Context(), request.PathValue("id"), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) SurveyPreflightHandler(response http.ResponseWriter, request *http.Request) {
	result, err := s.service.SurveyPreflight(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) CompleteSurveyHandler(response http.ResponseWriter, request *http.Request) {
	var meta application.CommandMeta
	if err := decodeJSON(response, request, &meta); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.CompleteSurvey(request.Context(), request.PathValue("id"), meta)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) GenerateRisksHandler(response http.ResponseWriter, request *http.Request) {
	var meta application.CommandMeta
	if err := decodeJSON(response, request, &meta); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.GenerateRisks(request.Context(), request.PathValue("id"), meta)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) AddRiskHandler(response http.ResponseWriter, request *http.Request) {
	var command application.AddRiskCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.AddManualRisk(request.Context(), request.PathValue("id"), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) DeleteRiskHandler(response http.ResponseWriter, request *http.Request) {
	var meta application.CommandMeta
	if err := decodeJSON(response, request, &meta); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.DeleteRisk(request.Context(), request.PathValue("id"), request.PathValue("riskId"), meta)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) BatchRiskUrgencyHandler(response http.ResponseWriter, request *http.Request) {
	var command application.BatchRiskUrgencyCommand
	if err := decodeJSON(response, request, &command); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.BatchAdjustRiskUrgency(request.Context(), request.PathValue("id"), command)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}
