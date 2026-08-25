package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

type PermitArchive struct {
	Restoration    *domain.RestorationCase
	Permit         domain.ReleasePermit
	Manifest       *domain.FrozenManifest
	PermitDigest   string
	ManifestDigest string
	StoredSerial   int64
}

func (s *Store) PermitSerialForCase(ctx context.Context, caseID string) (int64, error) {
	var serial int64
	err := s.db.QueryRowContext(ctx, `SELECT serial_number FROM release_permits WHERE case_id = ?`, caseID).Scan(&serial)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.NewError(domain.CodeNotFound, "作业 %s 尚无放行凭据", caseID)
	}
	return serial, err
}

func (s *Store) LoadByPermitSerial(ctx context.Context, serial int64) (PermitArchive, error) {
	var result PermitArchive
	var permitJSON []byte
	var caseID string
	var frozenVersion int64
	err := s.db.QueryRowContext(ctx, `SELECT serial_number, case_id, frozen_version, content_digest, permit_json FROM release_permits WHERE serial_number = ?`, serial).
		Scan(&result.StoredSerial, &caseID, &frozenVersion, &result.PermitDigest, &permitJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return result, domain.NewError(domain.CodeNotFound, "放行凭据编号 %d 不存在", serial)
	}
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(permitJSON, &result.Permit); err != nil {
		return result, domain.NewError(domain.CodeIntegrity, "编号 %d 的凭据归档损坏：%v", serial, err)
	}
	result.Restoration, err = scanCase(s.db.QueryRowContext(ctx, `SELECT aggregate_json FROM restoration_cases WHERE id = ?`, caseID))
	if errors.Is(err, sql.ErrNoRows) {
		return result, domain.NewError(domain.CodeIntegrity, "编号 %d 对应的作业聚合缺失", serial)
	}
	if err != nil {
		return result, domain.NewError(domain.CodeIntegrity, "编号 %d 对应的作业聚合损坏：%v", serial, err)
	}
	var manifestJSON []byte
	err = s.db.QueryRowContext(ctx, `SELECT content_digest, manifest_json FROM frozen_manifests WHERE case_id = ? AND frozen_version = ?`, caseID, frozenVersion).
		Scan(&result.ManifestDigest, &manifestJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	var manifest domain.FrozenManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return result, domain.NewError(domain.CodeIntegrity, "编号 %d 的冻结归档损坏：%v", serial, err)
	}
	result.Manifest = &manifest
	return result, nil
}
