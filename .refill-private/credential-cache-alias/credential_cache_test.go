package credential_cache_alias_test

import (
	"testing"
	"time"

	"paperfit-release/internal/application"
	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

func releasedCaseAndCredential(t *testing.T) (*domain.SuitabilityCase, domain.ReleaseCredential) {
	t.Helper()
	now := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	c, err := domain.NewCase("case-credential", "PF-CREDENTIAL", "古籍", "竹纸", "书页边缘", domain.PaperLot{
		PaperLotID: "lot-credential", Maker: "纸坊", Origin: "泾县", FiberComposition: "楮皮",
		ProductionDate: "2026-01-01", NominalWeightGSM: 35, SheetIdentifier: "sheet-1",
	}, "修复师", now)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.LockPlan("plan-1", "检测员", 1, c.Version, now); err != nil {
		t.Fatal(err)
	}
	measurements := []domain.Measurement{
		{MeasurementID: "m-ph", MetricCode: "ph", SampleID: "s1", Method: "标准法", Value: 7.2, Unit: "pH", MeasuredBy: "检测员"},
		{MeasurementID: "m-thickness", MetricCode: "thickness", SampleID: "s1", Method: "标准法", Value: 0.1, Unit: "mm", MeasuredBy: "检测员"},
		{MeasurementID: "m-grammage", MetricCode: "grammage", SampleID: "s1", Method: "标准法", Value: 35, Unit: "g/m2", MeasuredBy: "检测员"},
		{MeasurementID: "m-color", MetricCode: "color_difference", SampleID: "s1", Method: "标准法", Value: 2, Unit: "DeltaE", MeasuredBy: "检测员"},
		{MeasurementID: "m-fiber", MetricCode: "fiber_direction", SampleID: "s1", Method: "标准法", Value: 10, Unit: "degree", MeasuredBy: "检测员"},
		{MeasurementID: "m-wet", MetricCode: "wet_strength", SampleID: "s1", Method: "标准法", Value: 1, Unit: "N/15mm", MeasuredBy: "检测员"},
	}
	if err = c.AddMeasurementsBatch(measurements, c.Version, now); err != nil {
		t.Fatal(err)
	}
	if err = c.RecordTrial(domain.TrialAssessment{
		AssessmentID: "trial-1", PasteFormula: "1:5", WettingDuration: "30秒", DryingCondition: "压平24小时",
		ReversibilityGrade: "良好", AppearanceChange: "无", Decision: domain.DecisionUsable, EvidenceDigest: "digest-1",
	}, "修复师", c.Version, now); err != nil {
		t.Fatal(err)
	}
	if err = c.SubmitReview("修复师", "提交评审", c.Version, now); err != nil {
		t.Fatal(err)
	}
	confirmations := []domain.ReviewConfirmation{
		{Item: "material_traceability", Confirmed: true},
		{Item: "measurements", Confirmed: true},
		{Item: "trial", Confirmed: true},
		{Item: "risk_closure", Confirmed: true},
	}
	if err = c.Review("审核员", true, confirmations, nil, "评审通过", c.Version, now); err != nil {
		t.Fatal(err)
	}
	if err = c.Freeze("审核员", "冻结", c.Version, now); err != nil {
		t.Fatal(err)
	}
	precheck, err := c.CredentialPrecheck()
	if err != nil {
		t.Fatal(err)
	}
	credential, err := c.IssueCredentialScoped("PFC-ALIAS", precheck.RequiredScope, nil, "审核员", c.Version, now)
	if err != nil {
		t.Fatal(err)
	}
	return c, credential
}

func TestCredentialCacheMustIsolateMutableSlices(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, credential := releasedCaseAndCredential(t)
	if err = store.Commit(eventstore.CommitRequest{
		ExpectedVersion: 0, EventType: "credential.seeded", Case: c, Credential: &credential,
	}); err != nil {
		t.Fatal(err)
	}

	original := credential.UsageScope.ArtifactRefs[0]
	credential.UsageScope.ArtifactRefs[0] = "写入方污染"
	afterInputMutation, _ := store.Credential(credential.CredentialNumber)
	ingressPolluted := afterInputMutation.UsageScope.ArtifactRefs[0] != original
	credential.UsageScope.ArtifactRefs[0] = original

	returned, _ := store.Credential(credential.CredentialNumber)
	returned.UsageScope.ArtifactRefs[0] = "读取方污染"
	afterOutputMutation, _ := store.Credential(credential.CredentialNumber)
	egressPolluted := afterOutputMutation.UsageScope.ArtifactRefs[0] != original

	verification := application.NewService(store).VerifyCredential(credential.CredentialNumber)
	if ingressPolluted || egressPolluted || !verification.Valid {
		t.Fatalf("凭据缓存与调用方共享可变切片: ingress=%t egress=%t verification=%s", ingressPolluted, egressPolluted, verification.Status)
	}
}
