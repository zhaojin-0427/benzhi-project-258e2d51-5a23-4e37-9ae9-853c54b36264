package domain

import "time"

func (c *SuitabilityCase) RecordTrial(t TrialAssessment, actor string, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if c.State != StateTesting && c.State != StateRemediation {
		return NewError("invalid_state", "当前状态不允许记录模拟贴补试验")
	}
	if !c.TestingComplete() {
		return NewError("testing_incomplete", "必检项目尚未完成")
	}
	for field, value := range map[string]string{"assessmentID": t.AssessmentID, "pasteFormula": t.PasteFormula, "wettingDuration": t.WettingDuration, "dryingCondition": t.DryingCondition, "reversibilityGrade": t.ReversibilityGrade, "appearanceChange": t.AppearanceChange, "evidenceDigest": t.EvidenceDigest} {
		if err := requireText(value, field); err != nil {
			return err
		}
	}
	if t.Decision != DecisionUsable && t.Decision != DecisionRestricted && t.Decision != DecisionRejected {
		return NewError("invalid_decision", "未知适配结论")
	}
	hasBlockingRisk := false
	findingIDs := map[string]bool{}
	for i, f := range t.RiskFindings {
		if f.FindingID == "" || f.Level == "" || f.Description == "" || f.EvidenceSummary == "" {
			return NewError("validation_error", "riskFindings[%d] 信息不完整", i)
		}
		if findingIDs[f.FindingID] {
			return NewError("validation_error", "riskFindings[%d].findingID 重复", i)
		}
		findingIDs[f.FindingID] = true
		if f.Blocking {
			hasBlockingRisk = true
		}
	}
	if hasBlockingRisk && t.Decision == DecisionUsable {
		return NewError("risk_decision_conflict", "存在阻断风险时结论不能为可用")
	}
	for _, old := range c.TrialHistory {
		if old.AssessmentID == t.AssessmentID {
			return NewError("duplicate_assessment", "assessmentID 已存在")
		}
	}
	changes := []RiskChange{}
	if c.Trial == nil {
		if t.SupersedesAssessmentID != "" {
			return NewError("risk_review_error", "首次试验不得引用前序 assessmentID")
		}
		t.Revision = 1
		for _, f := range t.RiskFindings {
			changes = append(changes, RiskChange{FindingID: f.FindingID, Outcome: "new", AfterLevel: f.Level, EvidenceSummary: f.EvidenceSummary})
		}
	} else {
		if t.SupersedesAssessmentID != c.Trial.AssessmentID {
			return NewError("risk_review_error", "复测必须引用当前前序 assessmentID")
		}
		if t.RevisionReason == "" {
			return NewError("risk_review_error", "复测原因不能为空")
		}
		if t.EvidenceDigest == c.Trial.EvidenceDigest {
			return NewError("risk_review_error", "复测必须使用新的证据摘要")
		}
		t.Revision = c.Trial.Revision + 1
		newByID := map[string]RiskFinding{}
		for _, f := range t.RiskFindings {
			newByID[f.FindingID] = f
		}
		dispositions := map[string]RiskDisposition{}
		for _, d := range t.RiskDispositions {
			if d.FindingID == "" || (d.Outcome != "maintained" && d.Outcome != "downgraded" && d.Outcome != "closed") {
				return NewError("risk_review_error", "风险处置字段无效")
			}
			if _, exists := dispositions[d.FindingID]; exists {
				return NewError("risk_review_error", "findingID 风险处置重复")
			}
			dispositions[d.FindingID] = d
		}
		for _, old := range c.Trial.RiskFindings {
			if current, exists := newByID[old.FindingID]; exists {
				outcome := "maintained"
				if current.Level != old.Level || (old.Blocking && !current.Blocking) {
					outcome = "downgraded"
				}
				if d, ok := dispositions[old.FindingID]; ok && d.Outcome != "closed" {
					outcome = d.Outcome
				}
				changes = append(changes, RiskChange{FindingID: old.FindingID, Outcome: outcome, BeforeLevel: old.Level, AfterLevel: current.Level, EvidenceSummary: current.EvidenceSummary})
				continue
			}
			d, exists := dispositions[old.FindingID]
			if !exists || d.Outcome != "closed" || d.ClosureReason == "" || d.EvidenceSummary == "" {
				return NewError("risk_review_error", "上一修订风险 %s 缺少关闭说明和复测证据", old.FindingID)
			}
			changes = append(changes, RiskChange{FindingID: old.FindingID, Outcome: "closed", BeforeLevel: old.Level, ClosureReason: d.ClosureReason, EvidenceSummary: d.EvidenceSummary})
		}
		oldIDs := map[string]bool{}
		for _, f := range c.Trial.RiskFindings {
			oldIDs[f.FindingID] = true
		}
		for _, f := range t.RiskFindings {
			if !oldIDs[f.FindingID] {
				changes = append(changes, RiskChange{FindingID: f.FindingID, Outcome: "new", AfterLevel: f.Level, EvidenceSummary: f.EvidenceSummary})
			}
		}
	}
	t.CaseID = c.CaseID
	t.RiskChanges = changes
	t.RecordedBy, t.RecordedAt, t.RecordedVersion = actor, now, c.Version+1
	action := "record_trial"
	reason := "记录模拟贴补试验与风险判定"
	if t.Revision > 1 {
		action = "revise_trial"
		reason = t.RevisionReason
	}
	return c.change(action, RoleRestorer, actor, reason, now, func() error { c.Trial = &t; c.TrialHistory = append(c.TrialHistory, t); return nil })
}

func (c *SuitabilityCase) SubmitReview(actor, reason string, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if c.State != StateTesting && c.State != StateRemediation {
		return NewError("invalid_state", "当前状态不能提交评审")
	}
	if reason == "" {
		return NewError("validation_error", "送审原因不能为空")
	}
	if !c.TestingComplete() || c.Trial == nil {
		return NewError("candidate_incomplete", "检测和模拟贴补结论必须完整")
	}
	if len(c.OpenBlockingIssues()) > 0 {
		return NewError("open_blockers", "仍有未闭环的阻断问题")
	}
	if len(c.BlockingRiskFindings()) > 0 {
		return NewError("open_risk_findings", "模拟贴补试验仍有阻断风险")
	}
	round := c.newReviewRound(actor, reason, now)
	return c.change("submit_review", RoleRestorer, actor, reason, now, func() error {
		c.State = StateReview
		c.ReviewApproved = false
		c.ReviewRounds = append(c.ReviewRounds, round)
		return nil
	})
}

func (c *SuitabilityCase) ReviewBound(actor string, approved bool, confirmations []ReviewConfirmation, issues []ReviewIssue, reason string, expected, reviewRound int, submissionDigest string, now time.Time) error {
	if c.State != StateReview || len(c.ReviewRounds) == 0 {
		return NewError("review_round_conflict", "当前没有可审核的评审轮次")
	}
	current := c.ReviewRounds[len(c.ReviewRounds)-1]
	if reviewRound != current.ReviewRound || submissionDigest == "" || submissionDigest != current.SubmissionDigest {
		return NewError("review_round_conflict", "评审轮次或材料摘要与当前送审快照不一致")
	}
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	return c.Review(actor, approved, confirmations, issues, reason, expected, now)
}

func (c *SuitabilityCase) Review(actor string, approved bool, confirmations []ReviewConfirmation, issues []ReviewIssue, reason string, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	if c.State != StateReview {
		return NewError("invalid_state", "仅待评审档案可审核")
	}
	if reason == "" {
		return NewError("validation_error", "审核原因不能为空")
	}
	if approved {
		if len(issues) > 0 {
			return NewError("validation_error", "通过评审时不得附带整改问题")
		}
		required := map[string]bool{"material_traceability": false, "measurements": false, "trial": false, "risk_closure": false}
		for _, confirmation := range confirmations {
			if _, exists := required[confirmation.Item]; !exists {
				return NewError("validation_error", "未知评审确认项 %s", confirmation.Item)
			}
			if !confirmation.Confirmed {
				return NewError("review_unconfirmed", "评审项 %s 尚未确认", confirmation.Item)
			}
			if required[confirmation.Item] {
				return NewError("validation_error", "评审确认项 %s 重复", confirmation.Item)
			}
			required[confirmation.Item] = true
		}
		for item, confirmed := range required {
			if !confirmed {
				return NewError("review_unconfirmed", "缺少评审确认项 %s", item)
			}
		}
		return c.change("approve_review", RoleReviewer, actor, reason, now, func() error {
			c.ReviewApproved = true
			c.ReviewConfirmations = append([]ReviewConfirmation(nil), confirmations...)
			c.finishReviewRound("approved", actor, reason, confirmations, nil, now)
			return nil
		})
	}
	if len(issues) == 0 {
		return NewError("validation_error", "退回时必须提供结构化问题")
	}
	existingIssueIDs := map[string]bool{}
	for _, issue := range c.ReviewIssues {
		existingIssueIDs[issue.IssueID] = true
	}
	batchIssueIDs := map[string]bool{}
	for i := range issues {
		if issues[i].IssueID == "" || issues[i].Item == "" || issues[i].Opinion == "" {
			return NewError("validation_error", "reviewIssues[%d] 信息不完整", i)
		}
		if existingIssueIDs[issues[i].IssueID] || batchIssueIDs[issues[i].IssueID] {
			return NewError("validation_error", "reviewIssues[%d].issueID 重复", i)
		}
		batchIssueIDs[issues[i].IssueID] = true
		issues[i].Resolved = false
		issues[i].ReturnedAtVersion = c.Version + 1
		if len(c.ReviewRounds) > 0 {
			issues[i].ReviewRound = c.ReviewRounds[len(c.ReviewRounds)-1].ReviewRound
		}
		issues[i].Remediations = []RemediationProgress{}
	}
	return c.change("return_review", RoleReviewer, actor, reason, now, func() error {
		c.State = StateRemediation
		c.ReviewApproved = false
		c.ReviewConfirmations = append([]ReviewConfirmation(nil), confirmations...)
		c.ReviewIssues = append(c.ReviewIssues, issues...)
		c.finishReviewRound("returned", actor, reason, confirmations, issues, now)
		return nil
	})
}

func (c *SuitabilityCase) Remediate(issueID, measure, retest, actor string, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	if c.State != StateRemediation {
		return NewError("invalid_state", "仅整改中档案可登记整改")
	}
	if measure == "" || retest == "" {
		return NewError("validation_error", "整改措施和关联复测结果不能为空")
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
	if c.ReviewIssues[index].Resolved {
		return NewError("already_resolved", "评审问题已闭环")
	}
	return c.change("remediate_issue", RoleRestorer, actor, "完成整改并关联复测", now, func() error {
		c.ReviewIssues[index].Measure = measure
		c.ReviewIssues[index].RetestReference = retest
		c.ReviewIssues[index].Resolved = true
		return nil
	})
}
