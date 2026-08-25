package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

type VerificationResult struct {
	Valid            bool   `json:"valid"`
	SerialNumber     int64  `json:"serialNumber,omitempty"`
	ExpectedDigest   string `json:"expectedDigest,omitempty"`
	CalculatedDigest string `json:"calculatedDigest,omitempty"`
	Message          string `json:"message"`
}

type ArchiveVerificationInput struct {
	Restoration    *domain.RestorationCase
	Permit         domain.ReleasePermit
	Manifest       *domain.FrozenManifest
	PermitDigest   string
	ManifestDigest string
	StoredSerial   int64
}

func VerifyArchive(input ArchiveVerificationInput) VerificationResult {
	if input.Restoration == nil {
		return VerificationResult{Message: "凭据对应的作业聚合缺失"}
	}
	if input.Manifest == nil {
		return VerificationResult{SerialNumber: input.StoredSerial, Message: "凭据对应的冻结归档缺失"}
	}
	if input.Restoration.Permit == nil || input.Restoration.Frozen == nil {
		return VerificationResult{SerialNumber: input.StoredSerial, Message: "作业聚合缺少凭据或冻结清单"}
	}
	if input.StoredSerial != input.Permit.SerialNumber || input.StoredSerial != input.Restoration.Permit.SerialNumber {
		return VerificationResult{SerialNumber: input.StoredSerial, Message: "凭据编号与归档索引不一致"}
	}
	if input.Permit.CaseID != input.Restoration.ID || input.Manifest.CaseID != input.Restoration.ID {
		return VerificationResult{SerialNumber: input.StoredSerial, Message: "凭据、冻结归档与作业不一致"}
	}
	if input.Permit != *input.Restoration.Permit {
		return VerificationResult{SerialNumber: input.StoredSerial, Message: "凭据归档与作业聚合中的凭据不一致"}
	}
	if *input.Manifest != *input.Restoration.Frozen {
		return VerificationResult{SerialNumber: input.StoredSerial, Message: "冻结归档与作业聚合中的清单不一致"}
	}
	if input.Permit.FrozenVersion != input.Manifest.FrozenVersion || input.Permit.FrozenVersion != input.Restoration.Frozen.FrozenVersion {
		return VerificationResult{SerialNumber: input.StoredSerial, Message: "凭据冻结版本与归档不一致"}
	}
	if input.Permit.SchemaVersion != SchemaVersion || input.Manifest.SchemaVersion != SchemaVersion {
		return VerificationResult{SerialNumber: input.StoredSerial, Message: "凭据或冻结归档结构版本不匹配"}
	}
	if input.PermitDigest != input.Permit.ContentDigest || input.ManifestDigest != input.Manifest.ContentDigest ||
		input.Permit.ContentDigest != input.Manifest.ContentDigest || input.Manifest.ContentDigest != input.Restoration.Frozen.ContentDigest ||
		input.Manifest.CanonicalJSON != input.Restoration.Frozen.CanonicalJSON {
		return VerificationResult{SerialNumber: input.StoredSerial, ExpectedDigest: input.Manifest.ContentDigest, Message: "凭据、冻结归档与聚合摘要不一致"}
	}
	result := Verify(input.Restoration)
	result.SerialNumber = input.StoredSerial
	return result
}

func Verify(restoration *domain.RestorationCase) VerificationResult {
	if restoration == nil || restoration.Frozen == nil {
		return VerificationResult{Message: "缺少冻结清单"}
	}
	if restoration.Permit == nil {
		return VerificationResult{ExpectedDigest: restoration.Frozen.ContentDigest, Message: "尚未签发放行凭据"}
	}
	result := VerificationResult{SerialNumber: restoration.Permit.SerialNumber, ExpectedDigest: restoration.Frozen.ContentDigest}
	if restoration.Permit.FrozenVersion != restoration.Frozen.FrozenVersion {
		result.Message = "凭据冻结版本与清单不一致"
		return result
	}
	if restoration.Permit.SchemaVersion != restoration.Frozen.SchemaVersion || restoration.Permit.SchemaVersion != SchemaVersion {
		result.Message = "凭据结构版本不匹配"
		return result
	}
	var canonical any
	if err := json.Unmarshal([]byte(restoration.Frozen.CanonicalJSON), &canonical); err != nil {
		result.Message = "冻结规范化内容损坏"
		return result
	}
	digest := sha256.Sum256([]byte(restoration.Frozen.CanonicalJSON))
	archivedDigest := hex.EncodeToString(digest[:])
	_, currentDigest, err := Canonicalize(restoration)
	if err != nil {
		result.Message = "当前作业事实无法重新计算摘要"
		return result
	}
	result.CalculatedDigest = currentDigest
	if archivedDigest != restoration.Frozen.ContentDigest || currentDigest != restoration.Frozen.ContentDigest || restoration.Permit.ContentDigest != restoration.Frozen.ContentDigest {
		result.Message = "内容摘要不一致"
		return result
	}
	result.Valid = true
	result.Message = "凭据有效，冻结内容和版本摘要一致"
	return result
}
