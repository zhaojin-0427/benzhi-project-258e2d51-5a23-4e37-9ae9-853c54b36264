package restricted_freeze_deadend_test

import (
	"testing"
	"time"

	"paperfit-release/internal/domain"
)

func approvedRestrictedCase(t *testing.T) *domain.SuitabilityCase {
	t.Helper()
	now := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	c, err := domain.NewCase("case-restricted", "PF-RESTRICTED", "古籍", "竹纸", "书页边缘", domain.PaperLot{
		PaperLotID: "lot-restricted", Maker: "纸坊", Origin: "泾县", FiberComposition: "楮皮",
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
		ReversibilityGrade: "良好", AppearanceChange: "无", Decision: domain.DecisionRestricted, EvidenceDigest: "digest-1",
	}, "修复师", c.Version, now); err != nil {
		t.Fatal(err)
	}
	if err = c.SubmitReview("修复师", "提交限制使用结论", c.Version, now); err != nil {
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
	return c
}

func TestRestrictedDecisionWithoutTermsMustNotFreeze(t *testing.T) {
	c := approvedRestrictedCase(t)
	err := c.Freeze("审核员", "冻结", c.Version, time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC))
	if err == nil {
		precheck, precheckErr := c.CredentialPrecheck()
		if precheckErr != nil {
			t.Fatalf("restricted 档案已冻结但预检失败: %v", precheckErr)
		}
		_, issueErr := c.IssueCredentialScoped("PFC-1", precheck.RequiredScope, nil, "审核员", c.Version, time.Date(2026, 8, 25, 4, 0, 0, 0, time.UTC))
		if domain.ErrorCode(issueErr) == "restriction_missing" {
			t.Fatalf("restricted 档案在无任何可确认条款时进入不可修订的 frozen 状态")
		}
		t.Fatalf("无条款的 restricted 档案应在冻结前被拒绝，签发结果: %v", issueErr)
	}
	if c.State != domain.StateReview {
		t.Fatalf("冻结拒绝后应保留可纠正的 pending_review 状态，实际为 %s", c.State)
	}
}
