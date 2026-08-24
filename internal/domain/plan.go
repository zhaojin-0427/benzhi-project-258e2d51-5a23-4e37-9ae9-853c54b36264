package domain

import (
	"strings"
	"time"
)

const PlanRuleSetVersion = "paperfit-rules-2"

type PlanInputSummary struct {
	ArtifactRef      string   `json:"artifactRef"`
	ArtifactMaterial string   `json:"artifactMaterial"`
	RepairArea       string   `json:"repairArea"`
	PaperLot         PaperLot `json:"paperLot"`
	SampleCount      int      `json:"sampleCount"`
	RuleSetVersion   string   `json:"ruleSetVersion"`
}

type PlanPreview struct {
	CaseID          string           `json:"caseID"`
	CaseVersion     int              `json:"caseVersion"`
	RequiredMetrics []MetricRule     `json:"requiredMetrics"`
	SampleCount     int              `json:"sampleCount"`
	RuleSetVersion  string           `json:"ruleSetVersion"`
	InputSummary    PlanInputSummary `json:"inputSummary"`
	PreviewDigest   string           `json:"previewDigest"`
}

func buildPlanRules(c *SuitabilityCase, sampleCount int) ([]MetricRule, PlanInputSummary, error) {
	if c.State != StateDraft {
		return nil, PlanInputSummary{}, NewDetailedError("invalid_state", "仅草稿状态可预演或锁定检测方案", map[string]any{"currentState": c.State})
	}
	if sampleCount < 1 || sampleCount > 10 {
		return nil, PlanInputSummary{}, NewError("validation_error", "sampleCount 应为 1 至 10")
	}
	if strings.TrimSpace(c.ArtifactMaterial) == "" || strings.TrimSpace(c.RepairArea) == "" {
		return nil, PlanInputSummary{}, NewError("validation_error", "artifactMaterial 和 repairArea 不能为空")
	}
	if err := ValidatePaperLot(c.PaperLot); err != nil {
		return nil, PlanInputSummary{}, err
	}
	rules := []MetricRule{
		{Code: "ph", Name: "酸碱度", Unit: "pH", Minimum: 6.5, Maximum: 8.5, SampleCount: sampleCount, Derivation: []string{"基础规则：修复纸酸碱度 6.5 至 8.5"}},
		{Code: "thickness", Name: "厚度", Unit: "mm", Minimum: 0.03, Maximum: 0.30, SampleCount: sampleCount, Derivation: []string{"基础规则：修复纸厚度 0.03 至 0.30 mm"}},
		{Code: "grammage", Name: "定量", Unit: "g/m2", Minimum: c.PaperLot.NominalWeightGSM * 0.8, Maximum: c.PaperLot.NominalWeightGSM * 1.2, SampleCount: sampleCount, Derivation: []string{"候选纸标称定量上下浮动 20%"}},
		{Code: "color_difference", Name: "色差", Unit: "DeltaE", Minimum: 0, Maximum: 8, SampleCount: sampleCount, Derivation: []string{"基础规则：色差不高于 8 DeltaE"}},
		{Code: "fiber_direction", Name: "纤维方向", Unit: "degree", Minimum: 0, Maximum: 45, SampleCount: sampleCount, Derivation: []string{"基础规则：纤维方向偏差不高于 45 degree"}},
		{Code: "wet_strength", Name: "湿强度", Unit: "N/15mm", Minimum: 0.2, Maximum: 20, SampleCount: sampleCount, Derivation: []string{"基础规则：湿强度 0.2 至 20 N/15mm"}},
	}
	if strings.Contains(c.RepairArea, "书脊") {
		rules[4].Maximum = 30
		rules[4].Derivation = append(rules[4].Derivation, "书脊修复要求顺应折转方向，上限收紧为 30 degree")
	}
	if strings.Contains(c.ArtifactMaterial, "皮纸") || strings.Contains(c.ArtifactMaterial, "楮") {
		rules[5].Minimum = 0.3
		rules[5].Derivation = append(rules[5].Derivation, "皮纸或楮纸材质要求更高湿态承载，最低值提高为 0.3 N/15mm")
	}
	if strings.Contains(c.ArtifactMaterial, "竹纸") {
		rules[0].Maximum = 8.0
		rules[0].Derivation = append(rules[0].Derivation, "竹纸材质对碱性敏感，上限收紧为 8.0")
	}
	summary := PlanInputSummary{ArtifactRef: c.ArtifactRef, ArtifactMaterial: c.ArtifactMaterial, RepairArea: c.RepairArea, PaperLot: c.PaperLot, SampleCount: sampleCount, RuleSetVersion: PlanRuleSetVersion}
	return rules, summary, nil
}

func PreviewPlan(c *SuitabilityCase, sampleCount int) (PlanPreview, error) {
	rules, summary, err := buildPlanRules(c, sampleCount)
	if err != nil {
		return PlanPreview{}, err
	}
	return PlanPreview{CaseID: c.CaseID, CaseVersion: c.Version, RequiredMetrics: rules, SampleCount: sampleCount, RuleSetVersion: PlanRuleSetVersion, InputSummary: summary, PreviewDigest: HashJSON(summary)}, nil
}

func GeneratePlan(c *SuitabilityCase, planID, actor string, sampleCount int, now time.Time) (TestPlan, error) {
	preview, err := PreviewPlan(c, sampleCount)
	if err != nil {
		return TestPlan{}, err
	}
	return TestPlan{PlanID: planID, CaseID: c.CaseID, Revision: 1, RequiredMetrics: preview.RequiredMetrics, SampleCount: sampleCount, LockedAt: now, LockedBy: actor, RuleSetVersion: preview.RuleSetVersion}, nil
}

func (c *SuitabilityCase) LockPlanWithPreview(planID, actor string, sampleCount, expected int, previewDigest string, now time.Time) error {
	preview, err := PreviewPlan(c, sampleCount)
	if err != nil {
		return err
	}
	if previewDigest == "" {
		if err := c.checkVersion(expected); err != nil {
			return err
		}
		return NewError("plan_preview_stale", "检测方案预演摘要缺失或已过期")
	}
	if previewDigest != preview.PreviewDigest {
		return NewError("plan_preview_stale", "检测方案预演摘要缺失或已过期")
	}
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	plan, err := GeneratePlan(c, planID, actor, sampleCount, now)
	if err != nil {
		return err
	}
	return c.change("lock_test_plan", RoleTester, actor, "锁定必检项目和阈值", now, func() error { c.Plan = &plan; c.State = StateTesting; return nil })
}

// LockPlan 保留领域内旧调用兼容；公开入口使用 LockPlanWithPreview 强制复核摘要。
func (c *SuitabilityCase) LockPlan(planID, actor string, sampleCount, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	preview, err := PreviewPlan(c, sampleCount)
	if err != nil {
		return err
	}
	return c.LockPlanWithPreview(planID, actor, sampleCount, expected, preview.PreviewDigest, now)
}

func (c *SuitabilityCase) rule(metric string) (MetricRule, bool) {
	if c.Plan == nil {
		return MetricRule{}, false
	}
	for _, r := range c.Plan.RequiredMetrics {
		if r.Code == metric {
			return r, true
		}
	}
	return MetricRule{}, false
}
