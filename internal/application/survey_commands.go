package application

import (
	"context"
	"strings"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

func (s *Service) AddSurvey(ctx context.Context, caseID string, command AddSurveyCommand) (*domain.RestorationCase, error) {
	if command.ID == "" {
		command.ID = audit.NewID("survey")
	}
	return s.mutate(ctx, caseID, "SURVEY_RECORDED", command.CommandMeta, []domain.Role{domain.RolePatrol}, command,
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.AddObservation(domain.SurveyObservation{
				ID: command.ID, Area: command.Area, ConditionCode: command.ConditionCode,
				Severity: command.Severity, Extent: command.Extent, Notes: command.Notes,
				EvidenceRefs: command.EvidenceRefs, ObservedBy: command.Actor, ObservedAt: command.ObservedAt,
			}, now)
		}, map[string]any{"area": command.Area})
}

func (s *Service) CorrectSurvey(ctx context.Context, caseID string, command CorrectSurveyCommand) (*domain.RestorationCase, error) {
	if command.ID == "" {
		command.ID = audit.NewID("survey")
	}
	details := map[string]any{"reason": command.CorrectionReason, "area": command.Area}
	return s.mutate(ctx, caseID, "SURVEY_CORRECTED", command.CommandMeta, []domain.Role{domain.RolePatrol}, command,
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			err := restoration.CorrectObservation(domain.SurveyObservation{
				ID: command.ID, Area: command.Area, ConditionCode: command.ConditionCode,
				Severity: command.Severity, Extent: command.Extent, Notes: command.Notes,
				EvidenceRefs: command.EvidenceRefs, ObservedBy: command.Actor, ObservedAt: command.ObservedAt,
			}, command.SupersedesID, command.CorrectionReason, now)
			if err != nil {
				return err
			}
			for _, observation := range restoration.Surveys {
				if observation.ID == command.SupersedesID {
					details["originalRecord"] = observation
				}
				if observation.ID == command.ID {
					details["correctionRecord"] = observation
				}
			}
			return nil
		}, details)
}

func (s *Service) CompleteSurvey(ctx context.Context, caseID string, meta CommandMeta) (*domain.RestorationCase, error) {
	return s.mutate(ctx, caseID, "SURVEY_COMPLETED", meta, []domain.Role{domain.RolePatrol}, meta,
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.CompleteSurvey(now)
		}, nil)
}

func (s *Service) SurveyPreflight(ctx context.Context, caseID string) (domain.SurveyPreflight, error) {
	restoration, err := s.repository.Get(ctx, caseID)
	if err != nil {
		return domain.SurveyPreflight{}, err
	}
	return restoration.SurveyPreflight(), nil
}

func riskID(observation domain.SurveyObservation) string {
	replacer := strings.NewReplacer("survey-", "", "_", "-", " ", "-")
	return "risk-" + replacer.Replace(observation.ID)
}

func (s *Service) GenerateRisks(ctx context.Context, caseID string, meta CommandMeta) (*domain.RestorationCase, error) {
	return s.mutate(ctx, caseID, "RISKS_GENERATED", meta, []domain.Role{domain.RolePatrol, domain.RolePlanner}, meta,
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			_, err := restoration.GenerateRisks(now, func(observation domain.SurveyObservation) string {
				candidate := riskID(observation)
				for _, risk := range restoration.Risks {
					if risk.ID == candidate {
						return audit.NewID("risk")
					}
				}
				return candidate
			})
			return err
		}, nil)
}
