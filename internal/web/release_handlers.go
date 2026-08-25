package web

import (
	"net/http"
	"strconv"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

func (s *Server) FreezeHandler(response http.ResponseWriter, request *http.Request) {
	var meta application.CommandMeta
	if err := decodeJSON(response, request, &meta); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.Freeze(request.Context(), request.PathValue("id"), meta)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) VerifyPermitBySerialHandler(response http.ResponseWriter, request *http.Request) {
	serial, err := strconv.ParseInt(request.PathValue("serial"), 10, 64)
	if err != nil || serial <= 0 {
		writeError(response, domain.NewError(domain.CodeValidation, "凭据编号必须为正整数"))
		return
	}
	result, err := s.service.VerifyPermitBySerial(request.Context(), serial)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) ApproveHandler(response http.ResponseWriter, request *http.Request) {
	var meta application.CommandMeta
	if err := decodeJSON(response, request, &meta); err != nil {
		writeError(response, err)
		return
	}
	result, err := s.service.Approve(request.Context(), request.PathValue("id"), meta)
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (s *Server) VerifyPermitHandler(response http.ResponseWriter, request *http.Request) {
	result, err := s.service.VerifyPermit(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}
