package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

type Service struct {
	repository *store.Store
	now        func() time.Time
}

func New(repository *store.Store) *Service { return &Service{repository: repository, now: time.Now} }

func (s *Service) WithClock(clock func() time.Time) *Service {
	s.now = clock
	return s
}

func validateMeta(meta CommandMeta, allowed ...domain.Role) error {
	if strings.TrimSpace(meta.Actor) == "" {
		return domain.NewError(domain.CodeValidation, "操作者不能为空")
	}
	if strings.TrimSpace(meta.IdempotencyKey) == "" {
		return domain.NewError(domain.CodeValidation, "idempotencyKey 不能为空")
	}
	for _, role := range allowed {
		if meta.Role == role {
			return nil
		}
	}
	return domain.NewError(domain.CodeForbidden, "角色 %s 无权执行此操作", meta.Role)
}

// requestDigest 计算请求内容的规范化摘要。摘要只覆盖决定操作意图的字段，
// 排除 idempotencyKey 和 expectedVersion：idempotencyKey 是缓存键本身，
// expectedVersion 属于并发控制元数据。同一操作下若命令内容不同，摘要必须不同，
// 这样复用幂等键时可以识别内容冲突而不是错误重放缓存响应。
func requestDigest(payload any) string {
	if payload == nil {
		return ""
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var generic map[string]any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		return ""
	}
	delete(generic, "idempotencyKey")
	delete(generic, "expectedVersion")
	canonical, err := json.Marshal(generic)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func (s *Service) mutate(ctx context.Context, caseID, operation string, meta CommandMeta, allowed []domain.Role, requestPayload any, action func(*domain.RestorationCase, int64, time.Time) error, details map[string]any) (*domain.RestorationCase, error) {
	if err := validateMeta(meta, allowed...); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	digest := requestDigest(requestPayload)
	restoration, _, err := s.repository.TransactWithDigest(ctx, caseID, meta.ExpectedVersion, meta.IdempotencyKey, operation, digest,
		func(current *domain.RestorationCase, nextSerial int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			if current == nil {
				return nil, nil, domain.NewError(domain.CodeNotFound, "作业档案不存在")
			}
			next := current.Clone()
			before := next.Status
			if err := action(next, nextSerial, now); err != nil {
				return nil, nil, err
			}
			event := audit.Event(operation, meta.Actor, meta.Role, before, next.Status, next.Version, now, details)
			return next, &event, nil
		})
	return restoration, err
}

func (s *Service) Get(ctx context.Context, caseID string) (CaseDetails, error) {
	restoration, err := s.repository.Get(ctx, caseID)
	if err != nil {
		return CaseDetails{}, err
	}
	events, err := s.repository.AuditTrail(ctx, caseID)
	if err != nil {
		return CaseDetails{}, err
	}
	return CaseDetails{
		Case: restoration, Progress: BuildProgress(restoration), AuditTrail: events,
		AuditIntegrity: audit.VerifyTrail(restoration, events), ManifestSummary: audit.InspectManifest(restoration),
		Verification: audit.Verify(restoration), SurveyPreflight: restoration.SurveyPreflight(),
		RiskWorklist: BuildRiskWorklist(restoration), ReviewProgress: BuildReviewProgress(restoration),
		RemediationTodos: BuildRemediationTodos(restoration),
	}, nil
}

func (s *Service) List(ctx context.Context) ([]*domain.RestorationCase, error) {
	return s.repository.List(ctx)
}

func (s *Service) Integrity(ctx context.Context) (store.IntegrityStatus, error) {
	return s.repository.Integrity(ctx)
}
