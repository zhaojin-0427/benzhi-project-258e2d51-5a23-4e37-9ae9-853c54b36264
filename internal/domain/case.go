package domain

import (
	"fmt"
	"sort"
	"time"
)

type SuitabilityCase struct {
	CaseID              string               `json:"caseID"`
	CaseNumber          string               `json:"caseNumber"`
	ArtifactRef         string               `json:"artifactRef"`
	ArtifactMaterial    string               `json:"artifactMaterial"`
	RepairArea          string               `json:"repairArea"`
	PaperLotID          string               `json:"paperLotID"`
	PaperLot            PaperLot             `json:"paperLot"`
	State               State                `json:"state"`
	Version             int                  `json:"version"`
	CreatedBy           string               `json:"createdBy"`
	CreatedAt           time.Time            `json:"createdAt"`
	UpdatedAt           time.Time            `json:"updatedAt"`
	Plan                *TestPlan            `json:"testPlan,omitempty"`
	Measurements        []Measurement        `json:"measurements"`
	Trial               *TrialAssessment     `json:"trialAssessment,omitempty"`
	TrialHistory        []TrialAssessment    `json:"trialHistory"`
	ReviewIssues        []ReviewIssue        `json:"reviewIssues"`
	ReviewConfirmations []ReviewConfirmation `json:"reviewConfirmations"`
	ReviewApproved      bool                 `json:"reviewApproved"`
	ReviewRounds        []ReviewRound        `json:"reviewRounds"`
	FrozenDigest        string               `json:"frozenDigest,omitempty"`
	FrozenVersion       int                  `json:"frozenVersion,omitempty"`
	Audit               []AuditEntry         `json:"audit"`
}

func NewCase(id, number, artifactRef, material, area string, lot PaperLot, actor string, now time.Time) (*SuitabilityCase, error) {
	for field, value := range map[string]string{"caseID": id, "caseNumber": number, "artifactRef": artifactRef, "artifactMaterial": material, "repairArea": area, "createdBy": actor} {
		if err := requireText(value, field); err != nil {
			return nil, err
		}
	}
	if err := ValidatePaperLot(lot); err != nil {
		return nil, err
	}
	c := &SuitabilityCase{CaseID: id, CaseNumber: number, ArtifactRef: artifactRef, ArtifactMaterial: material, RepairArea: area, PaperLotID: lot.PaperLotID, PaperLot: lot, State: StateDraft, Version: 1, CreatedBy: actor, CreatedAt: now, UpdatedAt: now, Measurements: []Measurement{}, TrialHistory: []TrialAssessment{}, ReviewIssues: []ReviewIssue{}, ReviewConfirmations: []ReviewConfirmation{}, ReviewRounds: []ReviewRound{}, Audit: []AuditEntry{}}
	c.Audit = append(c.Audit, AuditEntry{Action: "create", Actor: actor, Role: RoleRestorer, Reason: "建立适配档案", At: now, BeforeVersion: 0, AfterVersion: 1})
	return c, nil
}

func (c *SuitabilityCase) UpdateDraft(artifactRef, material, repairArea string, lot PaperLot, actor, reason string, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	if c.State != StateDraft {
		return NewError("invalid_state", "仅草稿状态可补充基础资料")
	}
	for field, value := range map[string]string{"artifactRef": artifactRef, "artifactMaterial": material, "repairArea": repairArea} {
		if err := requireText(value, field); err != nil {
			return err
		}
	}
	if err := ValidatePaperLot(lot); err != nil {
		return err
	}
	return c.change("update_draft", RoleRestorer, actor, reason, now, func() error {
		c.ArtifactRef, c.ArtifactMaterial, c.RepairArea = artifactRef, material, repairArea
		c.PaperLot, c.PaperLotID = lot, lot.PaperLotID
		return nil
	})
}

func ValidatePaperLot(lot PaperLot) error {
	for field, value := range map[string]string{"paperLotID": lot.PaperLotID, "maker": lot.Maker, "origin": lot.Origin, "fiberComposition": lot.FiberComposition, "productionDate": lot.ProductionDate, "sheetIdentifier": lot.SheetIdentifier} {
		if err := requireText(value, field); err != nil {
			return err
		}
	}
	if lot.NominalWeightGSM <= 0 {
		return NewError("validation_error", "nominalWeightGSM 必须大于 0")
	}
	return nil
}

func (c *SuitabilityCase) ensureMutable() error {
	if c.State == StateFrozen || c.State == StateReleased {
		return NewError("case_frozen", "冻结后的档案不可修改")
	}
	return nil
}

func (c *SuitabilityCase) checkVersion(expected int) error {
	if expected != c.Version {
		return NewError("version_conflict", "expectedVersion=%d，当前版本=%d", expected, c.Version)
	}
	return nil
}

func (c *SuitabilityCase) change(action string, role Role, actor, reason string, now time.Time, mutate func() error) error {
	before := c.Version
	if err := mutate(); err != nil {
		return err
	}
	c.Version++
	c.UpdatedAt = now
	c.Audit = append(c.Audit, AuditEntry{Action: action, Actor: actor, Role: role, Reason: reason, At: now, BeforeVersion: before, AfterVersion: c.Version})
	return nil
}

func (c *SuitabilityCase) MissingMeasurements() []string {
	if c.Plan == nil {
		return []string{"test_plan"}
	}
	counts := map[string]map[string]bool{}
	for _, m := range c.Measurements {
		if counts[m.MetricCode] == nil {
			counts[m.MetricCode] = map[string]bool{}
		}
		counts[m.MetricCode][m.SampleID] = true
	}
	missing := []string{}
	for _, rule := range c.Plan.RequiredMetrics {
		if len(counts[rule.Code]) < rule.SampleCount {
			missing = append(missing, fmt.Sprintf("%s:%d/%d", rule.Code, len(counts[rule.Code]), rule.SampleCount))
		}
	}
	sort.Strings(missing)
	return missing
}

func (c *SuitabilityCase) OpenBlockingIssues() []ReviewIssue {
	result := []ReviewIssue{}
	for _, issue := range c.ReviewIssues {
		if issue.Blocking && !issue.Resolved {
			result = append(result, issue)
		}
	}
	return result
}

func (c *SuitabilityCase) BlockingRiskFindings() []RiskFinding {
	result := []RiskFinding{}
	if c.Trial == nil {
		return result
	}
	for _, finding := range c.Trial.RiskFindings {
		if finding.Blocking {
			result = append(result, finding)
		}
	}
	return result
}
