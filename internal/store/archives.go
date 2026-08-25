package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

func (s *Store) verifyAppendOnlyRecords(ctx context.Context, restoration *domain.RestorationCase) error {
	if restoration.Frozen == nil {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM frozen_manifests WHERE case_id = ?`, restoration.ID).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return domain.NewError(domain.CodeIntegrity, "作业聚合缺少已归档的冻结清单")
		}
	} else {
		var archived []byte
		var digest string
		err := s.db.QueryRowContext(ctx, `SELECT manifest_json, content_digest FROM frozen_manifests WHERE case_id = ? AND frozen_version = ?`, restoration.ID, restoration.Frozen.FrozenVersion).Scan(&archived, &digest)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.NewError(domain.CodeIntegrity, "冻结清单未保存到只追加归档")
		}
		if err != nil {
			return err
		}
		var manifest domain.FrozenManifest
		if err := json.Unmarshal(archived, &manifest); err != nil {
			return domain.NewError(domain.CodeIntegrity, "归档冻结清单损坏：%v", err)
		}
		if digest != restoration.Frozen.ContentDigest || manifest.ContentDigest != restoration.Frozen.ContentDigest || manifest.CanonicalJSON != restoration.Frozen.CanonicalJSON {
			return domain.NewError(domain.CodeIntegrity, "聚合冻结内容与只追加归档不一致")
		}
	}
	if restoration.Permit == nil {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM release_permits WHERE case_id = ?`, restoration.ID).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return domain.NewError(domain.CodeIntegrity, "作业聚合缺少已签发凭据")
		}
		return nil
	}
	var archived []byte
	var digest string
	var serial int64
	err := s.db.QueryRowContext(ctx, `SELECT permit_json, content_digest, serial_number FROM release_permits WHERE case_id = ? AND frozen_version = ?`, restoration.ID, restoration.Permit.FrozenVersion).Scan(&archived, &digest, &serial)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeIntegrity, "放行凭据未保存到不可变归档")
	}
	if err != nil {
		return err
	}
	var permit domain.ReleasePermit
	if err := json.Unmarshal(archived, &permit); err != nil {
		return domain.NewError(domain.CodeIntegrity, "归档放行凭据损坏：%v", err)
	}
	if serial != restoration.Permit.SerialNumber || permit.ID != restoration.Permit.ID || digest != restoration.Permit.ContentDigest || permit.ContentDigest != restoration.Permit.ContentDigest {
		return domain.NewError(domain.CodeIntegrity, "聚合放行凭据与不可变归档不一致")
	}
	return nil
}
