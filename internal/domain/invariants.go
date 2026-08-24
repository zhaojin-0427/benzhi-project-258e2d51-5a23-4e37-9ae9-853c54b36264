package domain

import (
	"fmt"
	"sort"
)

type MetricProgress struct {
	MetricCode      string   `json:"metricCode"`
	Name            string   `json:"name"`
	Unit            string   `json:"unit"`
	RequiredSamples int      `json:"requiredSamples"`
	ReceivedSamples int      `json:"receivedSamples"`
	SampleIDs       []string `json:"sampleIDs"`
	Complete        bool     `json:"complete"`
}

type RiskClosureStatus struct {
	FindingCount       int      `json:"findingCount"`
	BlockingFindings   int      `json:"blockingFindings"`
	ReviewIssueCount   int      `json:"reviewIssueCount"`
	OpenBlockingIssues int      `json:"openBlockingIssues"`
	OpenIssueIDs       []string `json:"openIssueIDs"`
	ReadyForReview     bool     `json:"readyForReview"`
}

func (c *SuitabilityCase) DetectionProgress() []MetricProgress {
	if c.Plan == nil {
		return []MetricProgress{}
	}
	samples := map[string]map[string]bool{}
	for _, measurement := range c.Measurements {
		if samples[measurement.MetricCode] == nil {
			samples[measurement.MetricCode] = map[string]bool{}
		}
		samples[measurement.MetricCode][measurement.SampleID] = true
	}
	result := make([]MetricProgress, 0, len(c.Plan.RequiredMetrics))
	for _, rule := range c.Plan.RequiredMetrics {
		ids := make([]string, 0, len(samples[rule.Code]))
		for sampleID := range samples[rule.Code] {
			ids = append(ids, sampleID)
		}
		sort.Strings(ids)
		result = append(result, MetricProgress{MetricCode: rule.Code, Name: rule.Name, Unit: rule.Unit, RequiredSamples: rule.SampleCount, ReceivedSamples: len(ids), SampleIDs: ids, Complete: len(ids) >= rule.SampleCount})
	}
	return result
}

func (c *SuitabilityCase) RiskStatus() RiskClosureStatus {
	status := RiskClosureStatus{ReviewIssueCount: len(c.ReviewIssues)}
	if c.Trial != nil {
		status.FindingCount = len(c.Trial.RiskFindings)
		for _, finding := range c.Trial.RiskFindings {
			if finding.Blocking {
				status.BlockingFindings++
			}
		}
	}
	for _, issue := range c.ReviewIssues {
		if issue.Blocking && !issue.Resolved {
			status.OpenBlockingIssues++
			status.OpenIssueIDs = append(status.OpenIssueIDs, issue.IssueID)
		}
	}
	sort.Strings(status.OpenIssueIDs)
	status.ReadyForReview = c.TestingComplete() && c.Trial != nil && status.OpenBlockingIssues == 0 && status.BlockingFindings == 0
	return status
}

func (c *SuitabilityCase) ValidateInvariant() error {
	if c.CaseID == "" || c.CaseNumber == "" {
		return fmt.Errorf("档案标识不完整")
	}
	if c.Version < 1 {
		return fmt.Errorf("档案版本必须为正数")
	}
	validState := c.State == StateDraft || c.State == StateTesting || c.State == StateReview || c.State == StateRemediation || c.State == StateFrozen || c.State == StateReleased
	if !validState {
		return fmt.Errorf("未知档案状态 %s", c.State)
	}
	if err := ValidatePaperLot(c.PaperLot); err != nil {
		return fmt.Errorf("纸批次无效: %w", err)
	}
	if c.PaperLotID != c.PaperLot.PaperLotID {
		return fmt.Errorf("档案与纸批次标识不一致")
	}
	if c.State != StateDraft && c.Plan == nil {
		return fmt.Errorf("非草稿档案缺少检测方案")
	}
	if c.Plan != nil {
		if c.Plan.CaseID != c.CaseID || c.Plan.PlanID == "" || len(c.Plan.RequiredMetrics) == 0 {
			return fmt.Errorf("检测方案引用无效")
		}
		seenRules := map[string]bool{}
		for _, rule := range c.Plan.RequiredMetrics {
			if rule.Code == "" || rule.Unit == "" || rule.Minimum > rule.Maximum || rule.SampleCount < 1 {
				return fmt.Errorf("检测规则 %s 无效", rule.Code)
			}
			if seenRules[rule.Code] {
				return fmt.Errorf("检测规则 %s 重复", rule.Code)
			}
			seenRules[rule.Code] = true
		}
	}
	measurementKeys := map[string]bool{}
	measurementIDs := map[string]bool{}
	for _, measurement := range c.Measurements {
		if measurement.CaseID != c.CaseID {
			return fmt.Errorf("测量 %s 引用错误档案", measurement.MeasurementID)
		}
		if measurementIDs[measurement.MeasurementID] {
			return fmt.Errorf("measurementID %s 重复", measurement.MeasurementID)
		}
		measurementIDs[measurement.MeasurementID] = true
		key := measurement.MetricCode + "\x00" + measurement.SampleID
		if measurementKeys[key] {
			return fmt.Errorf("检测项目和样本组合重复")
		}
		measurementKeys[key] = true
		rule, exists := c.rule(measurement.MetricCode)
		if !exists || measurement.Unit != rule.Unit || measurement.Value < rule.Minimum || measurement.Value > rule.Maximum {
			return fmt.Errorf("测量 %s 不符合锁定方案", measurement.MeasurementID)
		}
	}
	if c.Trial != nil && c.Trial.CaseID != c.CaseID {
		return fmt.Errorf("模拟贴补试验引用错误档案")
	}
	assessmentIDs := map[string]bool{}
	for index, trial := range c.TrialHistory {
		if trial.CaseID != c.CaseID || trial.AssessmentID == "" || trial.Revision != index+1 {
			return fmt.Errorf("第 %d 次模拟贴补修订链无效", index+1)
		}
		if assessmentIDs[trial.AssessmentID] {
			return fmt.Errorf("assessmentID %s 重复", trial.AssessmentID)
		}
		assessmentIDs[trial.AssessmentID] = true
		if index > 0 && trial.SupersedesAssessmentID != c.TrialHistory[index-1].AssessmentID {
			return fmt.Errorf("第 %d 次模拟贴补前序引用无效", index+1)
		}
	}
	if len(c.TrialHistory) > 0 && (c.Trial == nil || c.Trial.AssessmentID != c.TrialHistory[len(c.TrialHistory)-1].AssessmentID) {
		return fmt.Errorf("当前模拟贴补试验不是最新修订")
	}
	for index, round := range c.ReviewRounds {
		if round.ReviewRound != index+1 || round.SubmissionDigest == "" || round.SubmissionDigest != HashJSON(round.Material) {
			return fmt.Errorf("第 %d 轮评审快照无效", index+1)
		}
		if index < len(c.ReviewRounds)-1 && round.Status == "pending" {
			return fmt.Errorf("历史评审轮次仍为待评审")
		}
	}
	issueIDs := map[string]bool{}
	for _, issue := range c.ReviewIssues {
		if issue.IssueID == "" || issueIDs[issue.IssueID] {
			return fmt.Errorf("评审问题标识无效或重复")
		}
		issueIDs[issue.IssueID] = true
	}
	if c.State == StateReview || c.State == StateRemediation || c.State == StateFrozen || c.State == StateReleased {
		if !c.TestingComplete() || c.Trial == nil {
			return fmt.Errorf("评审后档案缺少完整检测或试验")
		}
	}
	if c.State == StateFrozen || c.State == StateReleased {
		if c.FrozenDigest == "" || c.FrozenVersion < 1 {
			return fmt.Errorf("冻结档案缺少摘要或版本")
		}
		if c.CurrentInputDigest() != c.FrozenDigest {
			return fmt.Errorf("冻结输入摘要不匹配")
		}
	}
	if len(c.Audit) != c.Version {
		return fmt.Errorf("审计记录数量与版本不一致")
	}
	for index, audit := range c.Audit {
		expectedBefore := index
		if audit.BeforeVersion != expectedBefore || audit.AfterVersion != expectedBefore+1 || audit.Actor == "" || audit.Action == "" {
			return fmt.Errorf("第 %d 条审计记录版本链无效", index+1)
		}
	}
	return nil
}
