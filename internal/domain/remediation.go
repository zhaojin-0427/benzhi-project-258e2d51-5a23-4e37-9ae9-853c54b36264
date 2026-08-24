package domain

import (
	"fmt"
	"strings"
	"time"
)

type ClosureMatrixRow struct {
	ReviewRound        int                  `json:"reviewRound"`
	IssueID            string               `json:"issueID"`
	Blocking           bool                 `json:"blocking"`
	Measure            string               `json:"measure,omitempty"`
	EvidenceReferences []EvidenceReference  `json:"evidenceReferences"`
	EvidenceResults    []EvidenceEvaluation `json:"evidenceResults"`
	Closed             bool                 `json:"closed"`
	UnmetConditions    []string             `json:"unmetConditions"`
}

func metricForIssue(issue ReviewIssue) string {
	if issue.MetricCode != "" {
		return issue.MetricCode
	}
	mapping := map[string]string{"酸碱度": "ph", "厚度": "thickness", "定量": "grammage", "色差": "color_difference", "纤维方向": "fiber_direction", "湿强度": "wet_strength"}
	for text, code := range mapping {
		if strings.Contains(issue.Item, text) {
			return code
		}
	}
	return ""
}

func requirementsForIssue(issue ReviewIssue) []string {
	if len(issue.EvidenceRequirements) > 0 {
		result := append([]string(nil), issue.EvidenceRequirements...)
		for i, requirement := range result {
			if requirement == "measurement" && metricForIssue(issue) != "" {
				result[i] = "measurement:" + metricForIssue(issue)
			}
			if requirement == "trial_assessment" {
				result[i] = "trial"
			}
		}
		return result
	}
	result := []string{}
	if code := metricForIssue(issue); code != "" {
		result = append(result, "measurement:"+code)
	}
	if strings.Contains(issue.Item, "回退") {
		result = append(result, "reversibility")
	}
	if strings.Contains(issue.Item, "外观") {
		result = append(result, "appearance")
	}
	if len(result) == 0 {
		result = append(result, "trial")
	}
	return result
}

func (c *SuitabilityCase) RemediateWithEvidence(issueID, measure, actor string, role Role, references []EvidenceReference, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	if c.State != StateRemediation {
		return NewError("invalid_state", "仅整改中档案可登记整改")
	}
	if measure == "" || len(references) == 0 {
		return NewError("validation_error", "整改措施和 evidenceReferences 不能为空")
	}
	index := -1
	for i := range c.ReviewIssues {
		if c.ReviewIssues[i].IssueID == issueID {
			index = i
			break
		}
	}
	if index < 0 {
		return NewError("not_found", "评审问题不存在")
	}
	issue := &c.ReviewIssues[index]
	requirements := requirementsForIssue(*issue)
	measurementRequirements := map[string]bool{}
	trialRequired := false
	for _, requirement := range requirements {
		if strings.HasPrefix(requirement, "measurement:") {
			measurementRequirements[strings.TrimPrefix(requirement, "measurement:")] = true
		}
		if requirement == "trial" || requirement == "reversibility" || requirement == "appearance" {
			trialRequired = true
		}
	}
	seen := map[string]bool{}
	for _, p := range issue.Remediations {
		for _, ref := range p.EvidenceReferences {
			seen[ref.Type+"\x00"+ref.ID] = true
		}
	}
	evaluations := make([]EvidenceEvaluation, 0, len(references))
	qualified := map[string]bool{}
	for i, ref := range references {
		key := ref.Type + "\x00" + ref.ID
		if ref.ID == "" || (ref.Type != "measurement" && ref.Type != "trial") {
			return NewError("evidence_invalid", "evidenceReferences[%d] 类型或标识无效", i)
		}
		if seen[key] {
			return NewError("evidence_invalid", "evidenceReferences[%d] 已登记", i)
		}
		seen[key] = true
		e := EvidenceEvaluation{Reference: ref}
		if ref.Type == "measurement" {
			var found *Measurement
			for j := range c.Measurements {
				if c.Measurements[j].MeasurementID == ref.ID {
					found = &c.Measurements[j]
					break
				}
			}
			if found == nil || found.RecordedVersion <= issue.ReturnedAtVersion {
				return NewError("evidence_invalid", "测量证据 %s 不属于本档案退回后的复测", ref.ID)
			}
			if !measurementRequirements[found.MetricCode] {
				return NewError("evidence_invalid", "测量证据 %s 的 metricCode 与问题不符", ref.ID)
			}
			e.Qualified, e.Result = true, fmt.Sprintf("%s=%.4g %s，符合锁定范围", found.MetricCode, found.Value, found.Unit)
			qualified["measurement:"+found.MetricCode] = true
		} else {
			if !trialRequired {
				return NewError("evidence_invalid", "问题不需要模拟贴补试验证据")
			}
			var found *TrialAssessment
			for j := range c.TrialHistory {
				if c.TrialHistory[j].AssessmentID == ref.ID {
					found = &c.TrialHistory[j]
					break
				}
			}
			if found == nil || found.RecordedVersion <= issue.ReturnedAtVersion {
				return NewError("evidence_invalid", "试验证据 %s 不属于本档案退回后的复测", ref.ID)
			}
			e.Qualified, e.Result = true, "试验修订证据摘要="+found.EvidenceDigest
			qualified["trial"], qualified["reversibility"], qualified["appearance"] = true, found.ReversibilityGrade != "", found.AppearanceChange != ""
		}
		evaluations = append(evaluations, e)
	}
	// 已保存的证据也参与累计闭环判断。
	for _, p := range issue.Remediations {
		for _, e := range p.EvidenceResults {
			if e.Qualified {
				if e.Reference.Type == "trial" {
					qualified["trial"], qualified["reversibility"], qualified["appearance"] = true, true, true
				} else {
					for _, m := range c.Measurements {
						if m.MeasurementID == e.Reference.ID {
							qualified["measurement:"+m.MetricCode] = true
						}
					}
				}
			}
		}
	}
	unmet := []string{}
	for _, requirement := range requirements {
		if !qualified[requirement] {
			unmet = append(unmet, requirement)
		}
	}
	progress := RemediationProgress{Measure: measure, ExecutedBy: actor, RecordedAt: now, EvidenceReferences: append([]EvidenceReference(nil), references...), EvidenceResults: evaluations, Closed: len(unmet) == 0, UnmetConditions: unmet, RecordedVersion: c.Version + 1}
	return c.change("remediate_issue", role, actor, "登记整改复测证据", now, func() error {
		issue.Measure = measure
		issue.Remediations = append(issue.Remediations, progress)
		issue.Resolved = len(unmet) == 0
		return nil
	})
}

func (c *SuitabilityCase) ClosureMatrix() []ClosureMatrixRow {
	rows := make([]ClosureMatrixRow, 0, len(c.ReviewIssues))
	for _, issue := range c.ReviewIssues {
		row := ClosureMatrixRow{ReviewRound: issue.ReviewRound, IssueID: issue.IssueID, Blocking: issue.Blocking, Closed: issue.Resolved, EvidenceReferences: []EvidenceReference{}, EvidenceResults: []EvidenceEvaluation{}}
		for _, p := range issue.Remediations {
			row.Measure = p.Measure
			row.EvidenceReferences = append(row.EvidenceReferences, p.EvidenceReferences...)
			row.EvidenceResults = append(row.EvidenceResults, p.EvidenceResults...)
			row.UnmetConditions = p.UnmetConditions
		}
		if len(issue.Remediations) == 0 {
			row.UnmetConditions = requirementsForIssue(issue)
		}
		rows = append(rows, row)
	}
	return rows
}
