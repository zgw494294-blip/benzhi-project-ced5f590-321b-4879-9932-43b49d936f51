package application

import (
	"context"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

func (s *Service) AddManualRisk(ctx context.Context, caseID string, command AddRiskCommand) (*domain.RestorationCase, error) {
	if command.ID == "" {
		command.ID = audit.NewID("risk")
	}
	return s.mutate(ctx, caseID, "RISK_ADDED", command.CommandMeta, []domain.Role{domain.RolePatrol, domain.RolePlanner}, command,
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.AddManualRisk(domain.RiskItem{ID: command.ID, Category: command.Category, Severity: command.Severity, Urgency: command.Urgency, Rationale: command.Rationale}, now)
		}, map[string]any{"category": command.Category})
}

func (s *Service) DeleteRisk(ctx context.Context, caseID, riskID string, meta CommandMeta) (*domain.RestorationCase, error) {
	payload := deleteRiskPayload{CommandMeta: meta, RiskID: riskID}
	return s.mutate(ctx, caseID, "RISK_DELETED", meta, []domain.Role{domain.RolePlanner}, payload,
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.DeleteRisk(riskID, now)
		}, map[string]any{"riskId": riskID})
}

func (s *Service) BatchAdjustRiskUrgency(ctx context.Context, caseID string, command BatchRiskUrgencyCommand) (*domain.RestorationCase, error) {
	details := map[string]any{}
	return s.mutate(ctx, caseID, "RISK_URGENCY_BATCH_ADJUSTED", command.CommandMeta, []domain.Role{domain.RolePlanner}, command,
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			changes, err := restoration.AdjustRiskUrgencies(command.Adjustments, now)
			if err == nil {
				details["adjustments"] = changes
			}
			return err
		}, details)
}

func (s *Service) UpsertMeasure(ctx context.Context, caseID string, command UpsertMeasureCommand) (*domain.RestorationCase, error) {
	if command.ID == "" {
		command.ID = audit.NewID("measure")
	}
	return s.mutate(ctx, caseID, "MEASURE_SAVED", command.CommandMeta, []domain.Role{domain.RolePlanner}, command,
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.UpsertMeasure(domain.TreatmentMeasure{
				ID: command.ID, RiskID: command.RiskID, Revision: command.Revision, Sequence: command.Sequence,
				Action: command.Action, Prohibitions: command.Prohibitions,
				AcceptanceCriteria: command.AcceptanceCriteria, PreparedBy: command.Actor,
			}, now)
		}, map[string]any{"riskId": command.RiskID, "revision": command.Revision})
}

func (s *Service) SubmitReview(ctx context.Context, caseID string, revision int, meta CommandMeta) (*domain.RestorationCase, error) {
	payload := submitReviewPayload{CommandMeta: meta, Revision: revision}
	return s.mutate(ctx, caseID, "REVIEW_SUBMITTED", meta, []domain.Role{domain.RolePlanner}, payload,
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.SubmitReview(revision, now)
		}, map[string]any{"revision": revision})
}
