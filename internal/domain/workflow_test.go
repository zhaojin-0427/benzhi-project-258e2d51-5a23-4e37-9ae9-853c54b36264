package domain

import (
	"testing"
	"time"
)

func newWorkflowCase(t *testing.T) *SuitabilityCase {
	t.Helper()
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	c, err := NewCase("case-1", "PF-1", "古籍-1", "竹纸", "书页边缘", PaperLot{PaperLotID: "lot-1", Maker: "纸坊", Origin: "泾县", FiberComposition: "楮皮", ProductionDate: "2026-01-01", NominalWeightGSM: 35, SheetIdentifier: "sheet-1"}, "修复师", now)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.LockPlan("plan-1", "检测员", 1, c.Version, now); err != nil {
		t.Fatal(err)
	}
	values := map[string]struct {
		unit  string
		value float64
	}{"ph": {"pH", 7.2}, "thickness": {"mm", 0.1}, "grammage": {"g/m2", 35}, "color_difference": {"DeltaE", 2}, "fiber_direction": {"degree", 10}, "wet_strength": {"N/15mm", 1}}
	for code, item := range values {
		err = c.AddMeasurement(Measurement{MeasurementID: "m-" + code, MetricCode: code, SampleID: "s1", Method: "标准法", Value: item.value, Unit: item.unit, MeasuredBy: "检测员"}, c.Version, now)
		if err != nil {
			t.Fatalf("添加 %s: %v", code, err)
		}
	}
	return c
}

func TestWorkflowRequiresRemediationClosure(t *testing.T) {
	c := newWorkflowCase(t)
	now := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	err := c.RecordTrial(TrialAssessment{AssessmentID: "trial-1", PasteFormula: "1:5", WettingDuration: "30秒", DryingCondition: "压平24小时", ReversibilityGrade: "良好", AppearanceChange: "无", Decision: DecisionUsable, EvidenceDigest: "digest"}, "修复师", c.Version, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.SubmitReview("修复师", "提交", c.Version, now); err != nil {
		t.Fatal(err)
	}
	issue := ReviewIssue{IssueID: "i1", Item: "回退性", Opinion: "补测", Blocking: true}
	if err = c.Review("审核员", false, nil, []ReviewIssue{issue}, "退回", c.Version, now); err != nil {
		t.Fatal(err)
	}
	if err = c.SubmitReview("修复师", "直接重提", c.Version, now); ErrorCode(err) != "open_blockers" {
		t.Fatalf("预期 open_blockers，得到 %v", err)
	}
	if err = c.Remediate("i1", "补做试验", "复测-1:合格", "修复师", c.Version, now); err != nil {
		t.Fatal(err)
	}
	if err = c.SubmitReview("修复师", "整改完成", c.Version, now); err != nil {
		t.Fatal(err)
	}
	confirmations := []ReviewConfirmation{{Item: "material_traceability", Confirmed: true}, {Item: "measurements", Confirmed: true}, {Item: "trial", Confirmed: true}, {Item: "risk_closure", Confirmed: true}}
	if err = c.Review("审核员", true, confirmations, nil, "通过", c.Version, now); err != nil {
		t.Fatal(err)
	}
	if err = c.Freeze("审核员", "冻结", c.Version, now); err != nil {
		t.Fatal(err)
	}
	credential, err := c.IssueCredential("PFC-1", "书页边缘", "审核员", c.Version, now)
	if err != nil {
		t.Fatal(err)
	}
	valid, status := VerifyCredential(credential, c)
	if !valid || status != "valid" {
		t.Fatalf("验真失败: %s", status)
	}
	if err = c.RecordTrial(*c.Trial, "修复师", c.Version, now); ErrorCode(err) != "case_frozen" {
		t.Fatalf("冻结后修改应失败: %v", err)
	}
}

func TestMeasurementValidation(t *testing.T) {
	c := newWorkflowCase(t)
	err := c.AddMeasurement(Measurement{MeasurementID: "duplicate", MetricCode: "ph", SampleID: "s1", Method: "标准法", Value: 7, Unit: "pH", MeasuredBy: "检测员"}, c.Version, time.Now())
	if ErrorCode(err) != "duplicate_sample" {
		t.Fatalf("预期重复样本错误，得到 %v", err)
	}
	bad := newWorkflowCase(t)
	err = bad.AddMeasurement(Measurement{MeasurementID: "bad", MetricCode: "ph", SampleID: "s2", Method: "标准法", Value: 7, Unit: "mm", MeasuredBy: "检测员"}, bad.Version, time.Now())
	if ErrorCode(err) != "invalid_unit" {
		t.Fatalf("预期单位错误，得到 %v", err)
	}
}
