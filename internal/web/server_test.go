package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	repository, err := store.Open("file:webtest?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return New(application.New(repository)).Handler()
}

func TestWorkspaceAndHealthAreServed(t *testing.T) {
	handler := testHandler(t)
	workspace := httptest.NewRecorder()
	handler.ServeHTTP(workspace, httptest.NewRequest(http.MethodGet, "/workspace", nil))
	if workspace.Code != http.StatusOK || !strings.Contains(workspace.Body.String(), "<body>") || !strings.Contains(workspace.Body.String(), "古树复壮作业放行台") || !strings.Contains(workspace.Body.String(), "按编号现场核验") {
		t.Fatalf("工作台页面无效：%d", workspace.Code)
	}
	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK || !strings.Contains(health.Body.String(), `"status":"ok"`) {
		t.Fatalf("健康检查失败：%d %s", health.Code, health.Body.String())
	}
}

func TestPermitSerialRouteValidatesAndUsesExactLookup(t *testing.T) {
	handler := testHandler(t)
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/v1/permits/0/verify", nil))
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "正整数") {
		t.Fatalf("非正凭据编号应被拒绝：%d %s", invalid.Code, invalid.Body.String())
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/v1/permits/999/verify", nil))
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "999") {
		t.Fatalf("不存在凭据编号应精确返回未找到：%d %s", missing.Code, missing.Body.String())
	}
}

func TestJSONRouteRejectsWrongContentType(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/cases", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "Content-Type") {
		t.Fatalf("错误映射不符合预期：%d %s", response.Code, response.Body.String())
	}
}
