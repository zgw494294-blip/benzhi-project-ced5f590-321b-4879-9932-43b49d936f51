package application

import (
	"context"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/audit"
	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

func (s *Service) ReviewMeasure(ctx context.Context, caseID string, command ReviewCommand) (*domain.RestorationCase, error) {
	if command.ID == "" {
		command.ID = audit.NewID("finding")
	}
	return s.mutate(ctx, caseID, "MEASURE_REVIEWED", command.CommandMeta, []domain.Role{domain.RoleReviewer},
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.RecordFinding(domain.ReviewFinding{
				ID: command.ID, MeasureID: command.MeasureID, Decision: command.Decision,
				Issue: command.Issue, Reviewer: command.Actor,
			}, now)
		}, map[string]any{"measureId": command.MeasureID, "decision": command.Decision})
}

func (s *Service) BatchReviewMeasures(ctx context.Context, caseID string, command BatchReviewCommand) (*domain.RestorationCase, error) {
	findings := make([]domain.ReviewFinding, len(command.Items))
	details := make([]map[string]any, len(command.Items))
	for i, item := range command.Items {
		if item.ID == "" {
			item.ID = audit.NewID("finding")
		}
		findings[i] = domain.ReviewFinding{ID: item.ID, MeasureID: item.MeasureID, Decision: item.Decision, Issue: item.Issue, Reviewer: command.Actor}
		details[i] = map[string]any{"measureId": item.MeasureID, "decision": item.Decision, "issue": item.Issue}
	}
	return s.mutate(ctx, caseID, "MEASURES_BATCH_REVIEWED", command.CommandMeta, []domain.Role{domain.RoleReviewer},
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.RecordFindings(findings, now)
		}, map[string]any{"items": details})
}

func (s *Service) AddRemediation(ctx context.Context, caseID string, command RemediationCommand) (*domain.RestorationCase, error) {
	return s.mutate(ctx, caseID, "REMEDIATION_EVIDENCE_ADDED", command.CommandMeta, []domain.Role{domain.RolePlanner},
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.AddRemediation(command.FindingID, command.Evidence, now)
		},
		map[string]any{"findingId": command.FindingID})
}

func (s *Service) VerifyRemediation(ctx context.Context, caseID string, command VerificationCommand) (*domain.RestorationCase, error) {
	return s.mutate(ctx, caseID, "REMEDIATION_VERIFIED", command.CommandMeta, []domain.Role{domain.RoleReviewer},
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			return restoration.VerifyRemediation(command.FindingID, command.Actor, command.Decision, now)
		},
		map[string]any{"findingId": command.FindingID, "decision": command.Decision})
}

func (s *Service) Freeze(ctx context.Context, caseID string, meta CommandMeta) (*domain.RestorationCase, error) {
	return s.mutate(ctx, caseID, "VERSION_FROZEN", meta, []domain.Role{domain.RoleReviewer},
		func(restoration *domain.RestorationCase, _ int64, now time.Time) error {
			manifest, err := audit.BuildManifest(restoration, meta.Actor, now)
			if err != nil {
				return err
			}
			return restoration.Freeze(manifest, now)
		}, nil)
}

func (s *Service) Approve(ctx context.Context, caseID string, meta CommandMeta) (*domain.RestorationCase, error) {
	return s.mutate(ctx, caseID, "PERMIT_ISSUED", meta, []domain.Role{domain.RoleReviewer},
		func(restoration *domain.RestorationCase, serial int64, now time.Time) error {
			if restoration.Frozen == nil {
				return domain.NewError(domain.CodeState, "缺少冻结版本")
			}
			return restoration.Release(domain.ReleasePermit{
				ID: audit.NewID("permit"), CaseID: restoration.ID, SerialNumber: serial,
				FrozenVersion: restoration.Frozen.FrozenVersion, ContentDigest: restoration.Frozen.ContentDigest,
				ApprovedBy: meta.Actor, SchemaVersion: audit.SchemaVersion,
			}, now)
		}, nil)
}

type PermitVerificationView struct {
	Valid            bool                    `json:"valid"`
	Message          string                  `json:"message"`
	SerialNumber     int64                   `json:"serialNumber"`
	ExpectedDigest   string                  `json:"expectedDigest,omitempty"`
	CalculatedDigest string                  `json:"calculatedDigest,omitempty"`
	WindowStatus     domain.WorkWindowStatus `json:"windowStatus"`
	ReadyToStart     bool                    `json:"readyToStart"`
	WorkWindowStart  time.Time               `json:"workWindowStart"`
	WorkWindowEnd    time.Time               `json:"workWindowEnd"`
	CaseID           string                  `json:"caseId"`
	TreeCode         string                  `json:"treeCode"`
	Location         string                  `json:"location"`
	Owner            string                  `json:"owner"`
	ApprovedBy       string                  `json:"approvedBy"`
	IssuedAt         time.Time               `json:"issuedAt"`
	FrozenVersion    int64                   `json:"frozenVersion"`
}

func (s *Service) VerifyPermit(ctx context.Context, caseID string) (PermitVerificationView, error) {
	serial, serialErr := s.repository.PermitSerialForCase(ctx, caseID)
	if serialErr == nil {
		return s.VerifyPermitBySerial(ctx, serial)
	}
	if domain.ErrorCodeOf(serialErr) != domain.CodeNotFound {
		return PermitVerificationView{}, serialErr
	}
	restoration, err := s.repository.Get(ctx, caseID)
	if err != nil {
		return PermitVerificationView{}, err
	}
	verification := audit.Verify(restoration)
	return PermitVerificationView{Valid: verification.Valid, Message: verification.Message, CaseID: restoration.ID, TreeCode: restoration.TreeCode}, nil
}

func (s *Service) VerifyPermitBySerial(ctx context.Context, serial int64) (PermitVerificationView, error) {
	if serial <= 0 {
		return PermitVerificationView{}, domain.NewError(domain.CodeValidation, "凭据编号必须为正整数")
	}
	if err := ctx.Err(); err != nil {
		return PermitVerificationView{}, err
	}
	// Reload the archive on every verification so that a corrupted or replaced
	// frozen-manifest archive is detected again instead of serving the stale
	// result from a prior successful verification. Only the work-window status
	// is derived from the current clock; the archive integrity is rechecked.
	archive, err := s.repository.LoadByPermitSerial(ctx, serial)
	if err != nil {
		return PermitVerificationView{}, err
	}
	verification := audit.VerifyArchive(audit.ArchiveVerificationInput{
		Restoration: archive.Restoration, Permit: archive.Permit, Manifest: archive.Manifest,
		PermitDigest: archive.PermitDigest, ManifestDigest: archive.ManifestDigest, StoredSerial: archive.StoredSerial,
	})
	caseRecord := archive.Restoration
	window := caseRecord.WorkWindowStatus(s.now())
	view := PermitVerificationView{
		Valid: verification.Valid, Message: verification.Message, SerialNumber: serial,
		ExpectedDigest: verification.ExpectedDigest, CalculatedDigest: verification.CalculatedDigest,
		WindowStatus: window, ReadyToStart: verification.Valid && window == domain.WindowActive,
		WorkWindowStart: caseRecord.WorkWindowStart, WorkWindowEnd: caseRecord.WorkWindowEnd,
		CaseID: caseRecord.ID, TreeCode: caseRecord.TreeCode, Location: caseRecord.Location, Owner: caseRecord.Owner,
		ApprovedBy: archive.Permit.ApprovedBy, IssuedAt: archive.Permit.IssuedAt, FrozenVersion: archive.Permit.FrozenVersion,
	}
	return view, nil
}
