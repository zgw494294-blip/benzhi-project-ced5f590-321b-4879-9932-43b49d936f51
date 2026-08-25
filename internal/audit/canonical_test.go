package audit

import (
	"testing"
	"time"

	"benzhi-project-ced5f590-321b-4879-9932-43b49d936f51/internal/domain"
)

func TestCanonicalizeStableAndDoesNotMutateEvidence(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	restoration := &domain.RestorationCase{
		ID: "case", TreeCode: "GS", WorkWindowStart: now, WorkWindowEnd: now.Add(time.Hour), SubmittedRevision: 1,
		Surveys:  []domain.SurveyObservation{{ID: "b", EvidenceRefs: []string{"z", "a"}}, {ID: "a", EvidenceRefs: []string{"c"}}},
		Risks:    []domain.RiskItem{{ID: "b"}, {ID: "a"}},
		Measures: []domain.TreatmentMeasure{{ID: "b", Sequence: 2}, {ID: "a", Sequence: 1}},
		Findings: []domain.ReviewFinding{{ID: "b"}, {ID: "a"}},
	}
	first, firstDigest, err := Canonicalize(restoration)
	if err != nil {
		t.Fatal(err)
	}
	second, secondDigest, err := Canonicalize(restoration)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) || firstDigest != secondDigest {
		t.Fatal("相同事实必须产生稳定摘要")
	}
	if restoration.Surveys[0].EvidenceRefs[0] != "z" {
		t.Fatal("规范化不得改变领域聚合中的证据顺序")
	}
}

func TestVerifyDetectsChangedFacts(t *testing.T) {
	now := time.Now().UTC()
	restoration := &domain.RestorationCase{ID: "case", TreeCode: "GS", WorkWindowStart: now, WorkWindowEnd: now.Add(time.Hour)}
	manifest, err := BuildManifest(restoration, "复核员", now)
	if err != nil {
		t.Fatal(err)
	}
	manifest.FrozenVersion = 2
	restoration.Frozen = &manifest
	restoration.Permit = &domain.ReleasePermit{SerialNumber: 1, FrozenVersion: 2, ContentDigest: manifest.ContentDigest, SchemaVersion: SchemaVersion}
	if !Verify(restoration).Valid {
		t.Fatal("未变更事实应核验通过")
	}
	restoration.Location = "被篡改的位置"
	if Verify(restoration).Valid {
		t.Fatal("当前事实与冻结清单不一致时必须核验失败")
	}
}
