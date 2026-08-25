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
	repository      *store.Store
	now             func() time.Time
	caseReadMu      sync.Mutex
	caseReadFlights map[string]*caseReadFlight
}

type caseReadFlight struct {
	done    chan struct{}
	details CaseDetails
	err     error
}

func New(repository *store.Store) *Service {
	return &Service{
		repository:      repository,
		now:             time.Now,
		caseReadFlights: make(map[string]*caseReadFlight),
	}
}

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
	return restoration, err
}

func (s *Service) Get(ctx context.Context, caseID string) (CaseDetails, error) {
	s.caseReadMu.Lock()
	if flight, exists := s.caseReadFlights[caseID]; exists {
		s.caseReadMu.Unlock()
		select {
		case <-flight.done:
			return flight.details, flight.err
		case <-ctx.Done():
			return CaseDetails{}, ctx.Err()
		}
	}
	flight := &caseReadFlight{done: make(chan struct{})}
	s.caseReadFlights[caseID] = flight
	s.caseReadMu.Unlock()

	details, err := s.loadCase(ctx, caseID)
	s.caseReadMu.Lock()
	flight.details, flight.err = details, err
	delete(s.caseReadFlights, caseID)
	close(flight.done)
	s.caseReadMu.Unlock()
	return details, err
}

func (s *Service) loadCase(ctx context.Context, caseID string) (CaseDetails, error) {
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
