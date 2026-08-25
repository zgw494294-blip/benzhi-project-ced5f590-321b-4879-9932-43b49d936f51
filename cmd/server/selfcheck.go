package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/application"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
	webadapter "benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/web"
)

type selfcheckClient struct {
	baseURL string
	client  *http.Client
	version int64
	keys    int
}

func (client *selfcheckClient) meta(actor string, role domain.Role) application.CommandMeta {
	client.keys++
	return application.CommandMeta{Actor: actor, Role: role, ExpectedVersion: client.version, IdempotencyKey: fmt.Sprintf("selfcheck-%03d", client.keys)}
}

func (client *selfcheckClient) request(method, path string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, client.baseURL+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	encoded, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%s %s 返回 %d：%s", method, path, response.StatusCode, string(encoded))
	}
	if target != nil && len(encoded) > 0 {
		return json.Unmarshal(encoded, target)
	}
	return nil
}

func (client *selfcheckClient) write(path string, payload any) (*domain.RestorationCase, error) {
	var restoration domain.RestorationCase
	if err := client.request(http.MethodPost, path, payload, &restoration); err != nil {
		return nil, err
	}
	client.version = restoration.Version
	return &restoration, nil
}

func runSelfcheck(address string) error {
	repository, err := store.Open("file:selfcheck?mode=memory&cache=shared")
	if err != nil {
		return err
	}
	defer repository.Close()
	service := application.New(repository)
	handler := webadapter.New(service).Handler()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("自检监听 %s: %w", address, err)
	}
	server := newHTTPServer(address, handler)
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	client := &selfcheckClient{baseURL: "http://" + listener.Addr().String(), client: &http.Client{Timeout: 8 * time.Second}}
	if err := executeSelfcheckFlow(client); err != nil {
		return err
	}
	select {
	case err := <-serveErrors:
		return fmt.Errorf("自检服务提前退出：%w", err)
	default:
	}
	fmt.Printf("自检通过：完整建档、调查、风险覆盖、复核、冻结、放行及凭据核验均成功（%s）\n", listener.Addr())
	return nil
}

func executeSelfcheckFlow(client *selfcheckClient) error {
	var health map[string]any
	if err := client.request(http.MethodGet, "/healthz", nil, &health); err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	created, err := client.write("/api/v1/cases", application.CreateCaseCommand{
		CommandMeta: client.meta("自检巡护员", domain.RolePatrol), ID: "case-selfcheck", TreeCode: "GS-SELF-001",
		Location: "自检保护样地", ProtectionGrade: "一级", Owner: "自检责任人",
		WorkWindowStart: now.Add(24 * time.Hour), WorkWindowEnd: now.Add(48 * time.Hour),
	})
	if err != nil {
		return err
	}
	client.version = created.Version
	areas := []domain.SurveyArea{domain.AreaCanopy, domain.AreaTrunk, domain.AreaRootZone, domain.AreaEnvironment}
	for index, area := range areas {
		command := application.AddSurveyCommand{
			CommandMeta: client.meta("自检巡护员", domain.RolePatrol), ID: fmt.Sprintf("survey-%d", index+1), Area: area,
			ConditionCode: fmt.Sprintf("OBS-%d", index+1), Severity: domain.SeverityMedium, Extent: "样地可测范围",
			Notes: "自检现场结构化调查事实", EvidenceRefs: []string{fmt.Sprintf("EVIDENCE-%d", index+1)}, ObservedAt: now,
		}
		if _, err := client.write("/api/v1/cases/case-selfcheck/surveys", command); err != nil {
			return err
		}
	}
	if _, err := client.write("/api/v1/cases/case-selfcheck/survey/complete", client.meta("自检巡护员", domain.RolePatrol)); err != nil {
		return err
	}
	generated, err := client.write("/api/v1/cases/case-selfcheck/risks/generate", client.meta("自检编制员", domain.RolePlanner))
	if err != nil {
		return err
	}
	for index, risk := range generated.Risks {
		command := application.UpsertMeasureCommand{
			CommandMeta: client.meta("自检编制员", domain.RolePlanner), ID: fmt.Sprintf("measure-%d", index+1),
			RiskID: risk.ID, Revision: 1, Sequence: index + 1, Action: "按风险边界实施复壮处置",
			Prohibitions: "禁止破坏健康组织和扩大作业边界", AcceptanceCriteria: "现场复测达到方案阈值并保留影像证据",
		}
		if _, err := client.write("/api/v1/cases/case-selfcheck/measures", command); err != nil {
			return err
		}
	}
	type submit struct {
		application.CommandMeta
		Revision int `json:"revision"`
	}
	submitted, err := client.write("/api/v1/cases/case-selfcheck/review/submit", submit{CommandMeta: client.meta("自检编制员", domain.RolePlanner), Revision: 1})
	if err != nil {
		return err
	}
	for index, measure := range submitted.Measures {
		command := application.ReviewCommand{
			CommandMeta: client.meta("自检复核员", domain.RoleReviewer), ID: fmt.Sprintf("finding-%d", index+1),
			MeasureID: measure.ID, Decision: domain.DecisionPass,
		}
		if _, err := client.write("/api/v1/cases/case-selfcheck/reviews", command); err != nil {
			return err
		}
	}
	frozen, err := client.write("/api/v1/cases/case-selfcheck/freeze", client.meta("自检复核负责人", domain.RoleReviewer))
	if err != nil {
		return err
	}
	if frozen.Frozen == nil || frozen.Frozen.ContentDigest == "" {
		return fmt.Errorf("自检冻结摘要缺失")
	}
	released, err := client.write("/api/v1/cases/case-selfcheck/approve", client.meta("自检复核负责人", domain.RoleReviewer))
	if err != nil {
		return err
	}
	if released.Permit == nil || released.Permit.SerialNumber != 1 {
		return fmt.Errorf("自检凭据未按预期签发")
	}
	var verification struct {
		Valid   bool   `json:"valid"`
		Message string `json:"message"`
	}
	if err := client.request(http.MethodGet, "/api/v1/cases/case-selfcheck/permit/verify", nil, &verification); err != nil {
		return err
	}
	if !verification.Valid {
		return fmt.Errorf("自检凭据核验失败：%s", verification.Message)
	}
	var details application.CaseDetails
	if err := client.request(http.MethodGet, "/api/v1/cases/case-selfcheck", nil, &details); err != nil {
		return err
	}
	if len(details.AuditTrail) < 13 {
		return fmt.Errorf("自检审计轨迹不完整：仅 %d 条", len(details.AuditTrail))
	}
	if !details.AuditIntegrity.Valid {
		return fmt.Errorf("自检审计连续性校验失败：%v", details.AuditIntegrity.Issues)
	}
	if !details.ManifestSummary.Available || details.ManifestSummary.RiskCount != len(released.Risks) {
		return fmt.Errorf("自检冻结清单统计不完整")
	}
	return nil
}
