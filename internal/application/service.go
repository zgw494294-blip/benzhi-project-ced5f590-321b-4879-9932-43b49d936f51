package application

import (
	"context"
	"strings"
	"sync"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/store"
)

type Service struct {
	repository *store.Store
	now        func() time.Time
	listMu     sync.RWMutex
	listCache  []*domain.RestorationCase
	listCached bool
}

func New(repository *store.Store) *Service { return &Service{repository: repository, now: time.Now} }

func (s *Service) WithClock(clock func() time.Time) *Service {
	s.now = clock
	return s
}

func (s *Service) invalidateListCache() {
	s.listMu.Lock()
	s.listCache = nil
	s.listCached = false
	s.listMu.Unlock()
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

func (s *Service) mutate(ctx context.Context, caseID, operation string, meta CommandMeta, allowed []domain.Role, action func(*domain.RestorationCase, int64, time.Time) error, details map[string]any) (*domain.RestorationCase, error) {
	if err := validateMeta(meta, allowed...); err != nil {
		return nil, err
	}
	now := s.now().UTC()
	restoration, _, err := s.repository.Transact(ctx, caseID, meta.ExpectedVersion, meta.IdempotencyKey, operation,
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
	if err == nil {
		s.invalidateListCache()
	}
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
	s.listMu.RLock()
	if s.listCached {
		items := make([]*domain.RestorationCase, len(s.listCache))
		for i, item := range s.listCache {
			items[i] = item.Clone()
		}
		s.listMu.RUnlock()
		return items, nil
	}
	s.listMu.RUnlock()

	s.listMu.Lock()
	defer s.listMu.Unlock()
	if s.listCached {
		items := make([]*domain.RestorationCase, len(s.listCache))
		for i, item := range s.listCache {
			items[i] = item.Clone()
		}
		return items, nil
	}
	items, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	s.listCache = items
	s.listCached = true
	result := make([]*domain.RestorationCase, len(items))
	for i, item := range items {
		result[i] = item.Clone()
	}
	return result, nil
}

func (s *Service) Integrity(ctx context.Context) (store.IntegrityStatus, error) {
	return s.repository.Integrity(ctx)
}
