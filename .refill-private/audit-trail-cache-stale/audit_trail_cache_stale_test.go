package audittrailcachestale_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/web"
)

func TestCaseDetailsReloadsAuditTrailAfterCommittedMutation(t *testing.T) {
	repository, err := store.Open("file:audit-trail-cache-stale?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	service := application.New(repository).WithClock(func() time.Time { return now })
	handler := web.New(service).Handler()

	createBody := `{
		"actor":"巡护员","role":"PATROL","expectedVersion":0,"idempotencyKey":"create-case",
		"id":"case-cache","treeCode":"GS-001","location":"东区","protectionGrade":"一级","owner":"养护组",
		"workWindowStart":"2026-08-25T09:00:00Z","workWindowEnd":"2026-08-25T18:00:00Z"
	}`
	serveJSON(t, handler, http.MethodPost, "/api/v1/cases", createBody, http.StatusCreated)

	first := readDetails(t, handler, "/api/v1/cases/case-cache")
	if len(first.AuditTrail) != 1 || !first.AuditIntegrity.Valid {
		t.Fatalf("初次详情应包含有效建档轨迹，得到 events=%d integrity=%+v", len(first.AuditTrail), first.AuditIntegrity)
	}

	surveyBody := `{
		"actor":"巡护员","role":"PATROL","expectedVersion":1,"idempotencyKey":"add-survey",
		"id":"survey-canopy","area":"CANOPY","conditionCode":"DEAD_BRANCH","severity":"HIGH",
		"extent":"树冠北侧","notes":"发现枯枝","evidenceRefs":["photo-001"],"observedAt":"2026-08-25T09:30:00Z"
	}`
	serveJSON(t, handler, http.MethodPost, "/api/v1/cases/case-cache/surveys", surveyBody, http.StatusOK)

	second := readDetails(t, handler, "/api/v1/cases/case-cache")
	if second.Case.Version != 2 {
		t.Fatalf("调查事务应已提交到版本 2，得到 %d", second.Case.Version)
	}
	if len(second.AuditTrail) != 2 || second.AuditTrail[1].Type != "SURVEY_RECORDED" || !second.AuditIntegrity.Valid {
		t.Fatalf("提交后详情必须同步返回最新审计轨迹，得到 events=%d integrity=%+v", len(second.AuditTrail), second.AuditIntegrity)
	}
}

func serveJSON(t *testing.T, handler http.Handler, method, target, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s 状态码=%d，响应=%s", method, target, response.Code, response.Body.String())
	}
	return response
}

func readDetails(t *testing.T, handler http.Handler, target string) application.CaseDetails {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s 状态码=%d，响应=%s", target, response.Code, response.Body.String())
	}
	var details application.CaseDetails
	if err := json.Unmarshal(response.Body.Bytes(), &details); err != nil {
		t.Fatal(err)
	}
	return details
}
