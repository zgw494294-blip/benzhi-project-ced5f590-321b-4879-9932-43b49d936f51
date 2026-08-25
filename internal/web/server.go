package web

import (
	"net/http"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
)

type Server struct {
	service *application.Service
	mux     *http.ServeMux
}

func New(service *application.Service) *Server {
	server := &Server{service: service, mux: http.NewServeMux()}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.HealthHandler)
	s.mux.HandleFunc("GET /workspace", s.WorkspaceHandler)
	s.mux.HandleFunc("GET /", s.RootHandler)
	s.mux.HandleFunc("GET /static/style.css", s.StyleHandler)
	s.mux.HandleFunc("GET /static/app.js", s.ScriptHandler)
	s.mux.HandleFunc("GET /api/v1/cases", s.ListCasesHandler)
	s.mux.HandleFunc("POST /api/v1/cases", s.CreateCaseHandler)
	s.mux.HandleFunc("GET /api/v1/cases/{id}", s.GetCaseHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/surveys", s.AddSurveyHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/surveys/corrections", s.CorrectSurveyHandler)
	s.mux.HandleFunc("GET /api/v1/cases/{id}/survey/preflight", s.SurveyPreflightHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/survey/complete", s.CompleteSurveyHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/risks/generate", s.GenerateRisksHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/risks", s.AddRiskHandler)
	s.mux.HandleFunc("DELETE /api/v1/cases/{id}/risks/{riskId}", s.DeleteRiskHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/risks/urgency/batch", s.BatchRiskUrgencyHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/measures", s.UpsertMeasureHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/review/submit", s.SubmitReviewHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/reviews", s.ReviewMeasureHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/reviews/batch", s.BatchReviewMeasuresHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/remediations", s.AddRemediationHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/remediations/verify", s.VerifyRemediationHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/freeze", s.FreezeHandler)
	s.mux.HandleFunc("POST /api/v1/cases/{id}/approve", s.ApproveHandler)
	s.mux.HandleFunc("GET /api/v1/cases/{id}/permit/verify", s.VerifyPermitHandler)
	s.mux.HandleFunc("GET /api/v1/permits/{serial}/verify", s.VerifyPermitBySerialHandler)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		response.Header().Set("Referrer-Policy", "same-origin")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:")
		next.ServeHTTP(response, request)
	})
}
