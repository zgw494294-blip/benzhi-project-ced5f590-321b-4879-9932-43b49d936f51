package application

import (
	"context"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

func (s *Service) CreateCase(ctx context.Context, command CreateCaseCommand) (*domain.RestorationCase, error) {
	if err := validateMeta(command.CommandMeta, domain.RolePatrol); err != nil {
		return nil, err
	}
	if command.ExpectedVersion != 0 {
		return nil, domain.NewError(domain.CodeConflict, "新建档案的 expectedVersion 必须为 0")
	}
	now := s.now().UTC()
	if command.ID == "" {
		command.ID = audit.NewID("case")
	}
	restoration, _, err := s.repository.Transact(ctx, command.ID, 0, command.IdempotencyKey, "CASE_CREATED",
		func(current *domain.RestorationCase, _ int64) (*domain.RestorationCase, *domain.AuditEvent, error) {
			if current != nil {
				return nil, nil, domain.NewError(domain.CodeDuplicate, "作业档案已经存在")
			}
			next, err := domain.NewCase(domain.NewCaseInput{
				ID: command.ID, TreeCode: command.TreeCode, Location: command.Location,
				ProtectionGrade: command.ProtectionGrade, Owner: command.Owner,
				WorkWindowStart: command.WorkWindowStart, WorkWindowEnd: command.WorkWindowEnd, Now: now,
			})
			if err != nil {
				return nil, nil, err
			}
			event := audit.Event("CASE_CREATED", command.Actor, command.Role, "", next.Status, next.Version, now, map[string]any{"treeCode": next.TreeCode})
			return next, &event, nil
		})
	return restoration, err
}
