package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

const SchemaVersion = "restoration-freeze/v1"

type canonicalCase struct {
	CaseID            string                     `json:"caseId"`
	TreeCode          string                     `json:"treeCode"`
	Location          string                     `json:"location"`
	ProtectionGrade   string                     `json:"protectionGrade"`
	Owner             string                     `json:"owner"`
	WorkWindowStart   time.Time                  `json:"workWindowStart"`
	WorkWindowEnd     time.Time                  `json:"workWindowEnd"`
	SubmittedRevision int                        `json:"submittedRevision"`
	Surveys           []domain.SurveyObservation `json:"surveys"`
	Risks             []domain.RiskItem          `json:"risks"`
	Measures          []domain.TreatmentMeasure  `json:"measures"`
	Findings          []domain.ReviewFinding     `json:"findings"`
}

func Canonicalize(restoration *domain.RestorationCase) ([]byte, string, error) {
	surveys := append([]domain.SurveyObservation(nil), restoration.Surveys...)
	risks := append([]domain.RiskItem(nil), restoration.Risks...)
	measures := append([]domain.TreatmentMeasure(nil), restoration.Measures...)
	findings := append([]domain.ReviewFinding(nil), restoration.Findings...)
	for i := range surveys {
		surveys[i].EvidenceRefs = append([]string(nil), surveys[i].EvidenceRefs...)
		sort.Strings(surveys[i].EvidenceRefs)
	}
	sort.Slice(surveys, func(i, j int) bool { return surveys[i].ID < surveys[j].ID })
	sort.Slice(risks, func(i, j int) bool { return risks[i].ID < risks[j].ID })
	sort.Slice(measures, func(i, j int) bool {
		if measures[i].Sequence == measures[j].Sequence {
			return measures[i].ID < measures[j].ID
		}
		return measures[i].Sequence < measures[j].Sequence
	})
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	payload := canonicalCase{
		CaseID: restoration.ID, TreeCode: restoration.TreeCode, Location: restoration.Location,
		ProtectionGrade: restoration.ProtectionGrade, Owner: restoration.Owner,
		WorkWindowStart: restoration.WorkWindowStart.UTC(), WorkWindowEnd: restoration.WorkWindowEnd.UTC(),
		SubmittedRevision: restoration.SubmittedRevision, Surveys: surveys, Risks: risks,
		Measures: measures, Findings: findings,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(encoded)
	return encoded, hex.EncodeToString(digest[:]), nil
}

func BuildManifest(restoration *domain.RestorationCase, actor string, now time.Time) (domain.FrozenManifest, error) {
	canonical, digest, err := Canonicalize(restoration)
	if err != nil {
		return domain.FrozenManifest{}, err
	}
	return domain.FrozenManifest{
		CaseID: restoration.ID, SchemaVersion: SchemaVersion, CanonicalJSON: string(canonical),
		ContentDigest: digest, FrozenBy: actor, FrozenAt: now.UTC(),
	}, nil
}
