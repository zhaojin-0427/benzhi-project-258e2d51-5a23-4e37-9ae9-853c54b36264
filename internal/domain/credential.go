package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type ReleaseCredential struct {
	CredentialNumber string            `json:"credentialNumber"`
	CaseID           string            `json:"caseID"`
	PaperLotID       string            `json:"paperLotID"`
	UsageScope       UsageScope        `json:"usageScope"`
	Restrictions     []RestrictionTerm `json:"restrictions"`
	Decision         Decision          `json:"decision"`
	FrozenVersion    int               `json:"frozenVersion"`
	InputDigest      string            `json:"inputDigest"`
	IssuedBy         string            `json:"issuedBy"`
	IssuedAt         time.Time         `json:"issuedAt"`
	CredentialHash   string            `json:"credentialHash"`
}

type UsageScope struct {
	ArtifactRef  string   `json:"artifactRef,omitempty"`
	RepairArea   string   `json:"repairArea,omitempty"`
	PaperLotID   string   `json:"paperLotID,omitempty"`
	ArtifactRefs []string `json:"artifactRefs"`
	RepairAreas  []string `json:"repairAreas"`
	PaperLotIDs  []string `json:"paperLotIDs"`
}

type RestrictionTerm struct {
	RestrictionID string `json:"restrictionID"`
	Text          string `json:"text"`
	Source        string `json:"source"`
}

type CredentialPrecheck struct {
	CaseID        string            `json:"caseID"`
	CaseVersion   int               `json:"caseVersion"`
	Decision      Decision          `json:"decision"`
	RequiredScope UsageScope        `json:"requiredScope"`
	Restrictions  []RestrictionTerm `json:"restrictions"`
	FrozenDigest  string            `json:"frozenDigest"`
}

func (c *SuitabilityCase) restrictionsForCredential() []RestrictionTerm {
	restrictions := []RestrictionTerm{}
	if c.Trial != nil {
		for _, f := range c.Trial.RiskFindings {
			if !f.Blocking {
				text := f.Description
				restrictions = append(restrictions, RestrictionTerm{RestrictionID: HashJSON(struct{ FindingID, Text string }{f.FindingID, text})[:16], Text: text, Source: "risk_finding:" + f.FindingID})
			}
		}
	}
	for _, confirmation := range c.ReviewConfirmations {
		if confirmation.Note != "" {
			restrictions = append(restrictions, RestrictionTerm{RestrictionID: HashJSON(struct{ Item, Note string }{confirmation.Item, confirmation.Note})[:16], Text: confirmation.Note, Source: "review_note:" + confirmation.Item})
		}
	}
	return restrictions
}

func (c *SuitabilityCase) CredentialPrecheck() (CredentialPrecheck, error) {
	if c.State != StateFrozen {
		return CredentialPrecheck{}, NewError("invalid_state", "仅已冻结档案可进行签发预检")
	}
	if c.CurrentInputDigest() != c.FrozenDigest {
		return CredentialPrecheck{}, NewError("frozen_tampered", "冻结输入摘要不一致")
	}
	restrictions := c.restrictionsForCredential()
	return CredentialPrecheck{CaseID: c.CaseID, CaseVersion: c.Version, Decision: c.Trial.Decision, RequiredScope: UsageScope{ArtifactRef: c.ArtifactRef, RepairArea: c.RepairArea, PaperLotID: c.PaperLotID, ArtifactRefs: []string{c.ArtifactRef}, RepairAreas: []string{c.RepairArea}, PaperLotIDs: []string{c.PaperLotID}}, Restrictions: restrictions, FrozenDigest: c.FrozenDigest}, nil
}

type digestInput struct {
	CaseID              string               `json:"caseID"`
	ArtifactRef         string               `json:"artifactRef"`
	RepairArea          string               `json:"repairArea"`
	PaperLot            PaperLot             `json:"paperLot"`
	Plan                *TestPlan            `json:"plan"`
	Measurements        []Measurement        `json:"measurements"`
	Trial               *TrialAssessment     `json:"trial"`
	ReviewIssues        []ReviewIssue        `json:"reviewIssues"`
	ReviewConfirmations []ReviewConfirmation `json:"reviewConfirmations"`
}

func HashJSON(value any) string {
	b, _ := json.Marshal(value)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func (c *SuitabilityCase) CurrentInputDigest() string {
	return HashJSON(digestInput{CaseID: c.CaseID, ArtifactRef: c.ArtifactRef, RepairArea: c.RepairArea, PaperLot: c.PaperLot, Plan: c.Plan, Measurements: c.Measurements, Trial: c.Trial, ReviewIssues: c.ReviewIssues, ReviewConfirmations: c.ReviewConfirmations})
}

func (c *SuitabilityCase) Freeze(actor, reason string, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	if c.State != StateReview || !c.ReviewApproved {
		return NewError("not_approved", "必须先通过技术评审")
	}
	if c.Trial == nil || (c.Trial.Decision != DecisionUsable && c.Trial.Decision != DecisionRestricted) {
		return NewError("not_releasable", "当前结论不可冻结放行")
	}
	if len(c.OpenBlockingIssues()) > 0 {
		return NewError("open_blockers", "仍有阻断问题")
	}
	if len(c.BlockingRiskFindings()) > 0 {
		return NewError("open_risk_findings", "仍有阻断风险发现")
	}
	if c.Trial.Decision == DecisionRestricted && len(c.restrictionsForCredential()) == 0 {
		return NewError("restriction_missing", "限制使用结论缺少可确认的限制条款")
	}
	digest := c.CurrentInputDigest()
	return c.change("freeze", RoleReviewer, actor, reason, now, func() error {
		c.State = StateFrozen
		c.FrozenDigest = digest
		c.FrozenVersion = c.Version + 1
		return nil
	})
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func scopeContains(single string, values []string, target string) bool {
	return single == target || containsString(values, target)
}

func validScopeDimension(single string, values []string, target string) bool {
	if single != "" && single != target {
		return false
	}
	if single == "" && len(values) == 0 {
		return false
	}
	for _, value := range values {
		if value != target {
			return false
		}
	}
	return single == target || containsString(values, target)
}

func (c *SuitabilityCase) IssueCredentialScoped(number string, scope UsageScope, confirmedRestrictions []string, actor string, expected int, now time.Time) (ReleaseCredential, error) {
	if err := c.checkVersion(expected); err != nil {
		return ReleaseCredential{}, err
	}
	if c.State != StateFrozen {
		return ReleaseCredential{}, NewError("invalid_state", "仅已冻结档案可签发凭据")
	}
	if c.CurrentInputDigest() != c.FrozenDigest {
		return ReleaseCredential{}, NewError("frozen_tampered", "冻结输入摘要不一致")
	}
	precheck, err := c.CredentialPrecheck()
	if err != nil {
		return ReleaseCredential{}, err
	}
	if !validScopeDimension(scope.ArtifactRef, scope.ArtifactRefs, c.ArtifactRef) || !validScopeDimension(scope.RepairArea, scope.RepairAreas, c.RepairArea) || !validScopeDimension(scope.PaperLotID, scope.PaperLotIDs, c.PaperLotID) {
		return ReleaseCredential{}, NewError("scope_invalid", "usageScope 未覆盖冻结档案的古籍、修复部位和纸批次")
	}
	knownRestrictions := map[string]bool{}
	for _, term := range precheck.Restrictions {
		knownRestrictions[term.RestrictionID] = true
	}
	seenConfirmations := map[string]bool{}
	for _, id := range confirmedRestrictions {
		if !knownRestrictions[id] || seenConfirmations[id] {
			return ReleaseCredential{}, NewError("restriction_conflict", "限制条款确认包含未知或重复标识")
		}
		seenConfirmations[id] = true
	}
	if c.Trial.Decision == DecisionRestricted && len(precheck.Restrictions) == 0 {
		return ReleaseCredential{}, NewError("restriction_missing", "限制使用结论缺少可确认的限制条款")
	}
	if c.Trial.Decision == DecisionRestricted {
		for _, term := range precheck.Restrictions {
			if !containsString(confirmedRestrictions, term.RestrictionID) {
				return ReleaseCredential{}, NewError("restriction_unconfirmed", "限制条款 %s 未确认", term.RestrictionID)
			}
		}
	}
	cred := ReleaseCredential{CredentialNumber: number, CaseID: c.CaseID, PaperLotID: c.PaperLotID, UsageScope: scope, Restrictions: precheck.Restrictions, Decision: c.Trial.Decision, FrozenVersion: c.FrozenVersion, InputDigest: c.FrozenDigest, IssuedBy: actor, IssuedAt: now}
	cred.CredentialHash = CredentialHash(cred)
	err = c.change("issue_credential", RoleReviewer, actor, "签发领用放行凭据", now, func() error { c.State = StateReleased; return nil })
	return cred, err
}

func (c *SuitabilityCase) IssueCredential(number, scope, actor string, expected int, now time.Time) (ReleaseCredential, error) {
	if scope == "" {
		return ReleaseCredential{}, NewError("validation_error", "usageScope 不能为空")
	}
	structured := UsageScope{ArtifactRef: c.ArtifactRef, RepairArea: c.RepairArea, PaperLotID: c.PaperLotID, ArtifactRefs: []string{c.ArtifactRef}, RepairAreas: []string{c.RepairArea}, PaperLotIDs: []string{c.PaperLotID}}
	precheck, _ := c.CredentialPrecheck()
	confirmations := []string{}
	for _, term := range precheck.Restrictions {
		confirmations = append(confirmations, term.RestrictionID)
	}
	return c.IssueCredentialScoped(number, structured, confirmations, actor, expected, now)
}

func CredentialHash(c ReleaseCredential) string {
	c.CredentialHash = ""
	return HashJSON(c)
}

func VerifyCredential(c ReleaseCredential, source *SuitabilityCase) (bool, string) {
	if CredentialHash(c) != c.CredentialHash {
		return false, "credential_content_mismatch"
	}
	if source == nil {
		return false, "case_not_found"
	}
	if source.FrozenDigest != c.InputDigest || source.CurrentInputDigest() != c.InputDigest {
		return false, "frozen_content_mismatch"
	}
	return true, "valid"
}

func VerifyCredentialScope(c ReleaseCredential, artifactRef, repairArea, paperLotID string) (string, []string) {
	mismatches := []string{}
	if artifactRef == "" && repairArea == "" && paperLotID == "" {
		return "not_checked", mismatches
	}
	if artifactRef != "" && !scopeContains(c.UsageScope.ArtifactRef, c.UsageScope.ArtifactRefs, artifactRef) {
		mismatches = append(mismatches, "artifactRef")
	}
	if repairArea != "" && !scopeContains(c.UsageScope.RepairArea, c.UsageScope.RepairAreas, repairArea) {
		mismatches = append(mismatches, "repairArea")
	}
	if paperLotID != "" && !scopeContains(c.UsageScope.PaperLotID, c.UsageScope.PaperLotIDs, paperLotID) {
		mismatches = append(mismatches, "paperLotID")
	}
	if len(mismatches) > 0 {
		return "out_of_scope", mismatches
	}
	return "matched", mismatches
}
