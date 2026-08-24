package application

import (
	"paperfit-release/internal/domain"
)

type AvailableActions struct {
	UpdateDraft     bool `json:"updateDraft"`
	LockTestPlan    bool `json:"lockTestPlan"`
	AddMeasurement  bool `json:"addMeasurement"`
	RecordTrial     bool `json:"recordTrial"`
	SubmitReview    bool `json:"submitReview"`
	Review          bool `json:"review"`
	Remediate       bool `json:"remediate"`
	Freeze          bool `json:"freeze"`
	IssueCredential bool `json:"issueCredential"`
}

type ReadinessReport struct {
	CaseID              string                    `json:"caseID"`
	State               domain.State              `json:"state"`
	Version             int                       `json:"version"`
	Actions             AvailableActions          `json:"actions"`
	MissingMeasurements []string                  `json:"missingMeasurements"`
	RiskStatus          domain.RiskClosureStatus  `json:"riskStatus"`
	Blockers            []string                  `json:"blockers"`
	ClosureMatrix       []domain.ClosureMatrixRow `json:"closureMatrix"`
}

func (s *Service) Readiness(id string) (ReadinessReport, error) {
	c, err := s.load(id)
	if err != nil {
		return ReadinessReport{}, err
	}
	report := ReadinessReport{CaseID: c.CaseID, State: c.State, Version: c.Version, MissingMeasurements: c.MissingMeasurements(), RiskStatus: c.RiskStatus(), Blockers: []string{}, ClosureMatrix: c.ClosureMatrix()}
	switch c.State {
	case domain.StateDraft:
		report.Actions.UpdateDraft = true
		report.Actions.LockTestPlan = true
		report.Blockers = append(report.Blockers, "检测方案尚未锁定")
	case domain.StateTesting:
		report.Actions.AddMeasurement = true
		if c.TestingComplete() {
			report.Actions.RecordTrial = true
		} else {
			report.Blockers = append(report.Blockers, "必检样本尚未齐全")
		}
		if c.TestingComplete() && c.Trial != nil && len(c.BlockingRiskFindings()) == 0 {
			report.Actions.SubmitReview = true
		} else if c.Trial == nil {
			report.Blockers = append(report.Blockers, "模拟贴补试验尚未记录")
		}
	case domain.StateReview:
		report.Actions.Review = true
		if c.ReviewApproved && len(c.OpenBlockingIssues()) == 0 && c.Trial != nil && (c.Trial.Decision == domain.DecisionUsable || c.Trial.Decision == domain.DecisionRestricted) {
			report.Actions.Freeze = true
		} else if !c.ReviewApproved {
			report.Blockers = append(report.Blockers, "技术评审尚未通过")
		}
	case domain.StateRemediation:
		report.Actions.AddMeasurement = true
		report.Actions.RecordTrial = c.TestingComplete()
		report.Actions.Remediate = len(c.OpenBlockingIssues()) > 0
		if len(c.OpenBlockingIssues()) == 0 && len(c.BlockingRiskFindings()) == 0 && c.TestingComplete() && c.Trial != nil {
			report.Actions.SubmitReview = true
		} else {
			report.Blockers = append(report.Blockers, "整改阻断项尚未全部闭环")
		}
	case domain.StateFrozen:
		report.Actions.IssueCredential = c.CurrentInputDigest() == c.FrozenDigest
		if !report.Actions.IssueCredential {
			report.Blockers = append(report.Blockers, "冻结输入摘要不一致")
		}
	case domain.StateReleased:
		report.Blockers = append(report.Blockers, "档案已签发领用凭据")
	}
	return report, nil
}

type AuditQuery struct {
	AfterVersion int
	Action       string
	Role         domain.Role
}

type AuditTrail struct {
	CaseID         string              `json:"caseID"`
	CurrentVersion int                 `json:"currentVersion"`
	Entries        []domain.AuditEntry `json:"entries"`
	Count          int                 `json:"count"`
}

func (s *Service) AuditTrail(id string, query AuditQuery) (AuditTrail, error) {
	c, err := s.load(id)
	if err != nil {
		return AuditTrail{}, err
	}
	entries := make([]domain.AuditEntry, 0, len(c.Audit))
	for _, entry := range c.Audit {
		if entry.AfterVersion <= query.AfterVersion {
			continue
		}
		if query.Action != "" && entry.Action != query.Action {
			continue
		}
		if query.Role != "" && entry.Role != query.Role {
			continue
		}
		entries = append(entries, entry)
	}
	return AuditTrail{CaseID: c.CaseID, CurrentVersion: c.Version, Entries: entries, Count: len(entries)}, nil
}
