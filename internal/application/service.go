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
	done chan struct{}

	mu         sync.Mutex
	details    CaseDetails
	err        error
	alive      int
	cancelRead context.CancelFunc
}

// register links a caller's context to this flight's lifetime. The shared
// background read is canceled only once every registered caller has canceled,
// so a single caller's cancellation never aborts the read for the others.
// The returned stop function must be called when the caller leaves on its own
// (i.e. it observed flight.done) so the context is no longer tracked.
func (f *caseReadFlight) register(ctx context.Context) func() bool {
	f.mu.Lock()
	f.alive++
	f.mu.Unlock()
	stop := context.AfterFunc(ctx, func() {
		f.mu.Lock()
		f.alive--
		if f.alive <= 0 {
			select {
			case <-f.done:
			default:
				f.cancelRead()
			}
		}
		f.mu.Unlock()
	})
	return stop
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
	for {
		s.caseReadMu.Lock()
		if flight, exists := s.caseReadFlights[caseID]; exists {
			select {
			case <-flight.done:
				// The in-flight read already finished; start a fresh read below
				// rather than reusing a result that may have been aborted by
				// every prior caller canceling.
			default:
				stop := flight.register(ctx)
				s.caseReadMu.Unlock()
				select {
				case <-flight.done:
					stop()
					details, err := flight.details, flight.err
					if err == nil || ctx.Err() != nil {
						return details, err
					}
					// The shared read was aborted (e.g. all callers canceled)
					// while this caller is still valid: retry on a new flight.
					continue
				case <-ctx.Done():
					return CaseDetails{}, ctx.Err()
				}
			}
		}
		flight := &caseReadFlight{done: make(chan struct{})}
		readCtx, cancelRead := context.WithCancel(context.Background())
		flight.cancelRead = cancelRead
		s.caseReadFlights[caseID] = flight
		stop := flight.register(ctx)
		s.caseReadMu.Unlock()

		go func() {
			details, err := s.loadCase(readCtx, caseID)
			flight.mu.Lock()
			flight.details, flight.err = details, err
			close(flight.done)
			flight.mu.Unlock()
			s.caseReadMu.Lock()
			delete(s.caseReadFlights, caseID)
			s.caseReadMu.Unlock()
			cancelRead()
		}()

		select {
		case <-flight.done:
			stop()
			details, err := flight.details, flight.err
			if err == nil || ctx.Err() != nil {
				return details, err
			}
			// The shared read was aborted while this caller is still valid:
			// retry on a new flight.
			continue
		case <-ctx.Done():
			return CaseDetails{}, ctx.Err()
		}
	}
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
