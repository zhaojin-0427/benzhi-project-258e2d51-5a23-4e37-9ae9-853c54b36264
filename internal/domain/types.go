package domain

import "time"

type State string

const (
	StateDraft       State = "draft"
	StateTesting     State = "testing"
	StateReview      State = "pending_review"
	StateRemediation State = "remediation"
	StateFrozen      State = "frozen"
	StateReleased    State = "released"
)

type Role string

const (
	RoleRestorer Role = "restorer"
	RoleTester   Role = "tester"
	RoleReviewer Role = "reviewer"
)

type Decision string

const (
	DecisionUsable     Decision = "usable"
	DecisionRestricted Decision = "restricted"
	DecisionRejected   Decision = "rejected"
)

type PaperLot struct {
	PaperLotID       string  `json:"paperLotID"`
	Maker            string  `json:"maker"`
	Origin           string  `json:"origin"`
	FiberComposition string  `json:"fiberComposition"`
	ProductionDate   string  `json:"productionDate"`
	NominalWeightGSM float64 `json:"nominalWeightGSM"`
	SheetIdentifier  string  `json:"sheetIdentifier"`
	SourceNote       string  `json:"sourceNote,omitempty"`
}

type MetricRule struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Unit        string   `json:"unit"`
	Minimum     float64  `json:"minimum"`
	Maximum     float64  `json:"maximum"`
	SampleCount int      `json:"sampleCount"`
	Derivation  []string `json:"derivation"`
}

type TestPlan struct {
	PlanID          string       `json:"planID"`
	CaseID          string       `json:"caseID"`
	Revision        int          `json:"revision"`
	RequiredMetrics []MetricRule `json:"requiredMetrics"`
	SampleCount     int          `json:"sampleCount"`
	LockedAt        time.Time    `json:"lockedAt"`
	LockedBy        string       `json:"lockedBy"`
	RuleSetVersion  string       `json:"ruleSetVersion"`
}

type Measurement struct {
	MeasurementID   string    `json:"measurementID"`
	CaseID          string    `json:"caseID"`
	MetricCode      string    `json:"metricCode"`
	SampleID        string    `json:"sampleID"`
	Method          string    `json:"method"`
	Value           float64   `json:"value"`
	Unit            string    `json:"unit"`
	MeasuredBy      string    `json:"measuredBy"`
	MeasuredAt      time.Time `json:"measuredAt"`
	RecordedVersion int       `json:"recordedVersion"`
}

type RiskFinding struct {
	FindingID       string `json:"findingID"`
	Level           string `json:"level"`
	Description     string `json:"description"`
	EvidenceSummary string `json:"evidenceSummary"`
	Blocking        bool   `json:"blocking"`
}

type RiskDisposition struct {
	FindingID       string `json:"findingID"`
	Outcome         string `json:"outcome"`
	Level           string `json:"level,omitempty"`
	Description     string `json:"description,omitempty"`
	EvidenceSummary string `json:"evidenceSummary,omitempty"`
	Blocking        bool   `json:"blocking,omitempty"`
	ClosureReason   string `json:"closureReason,omitempty"`
}

type RiskChange struct {
	FindingID       string `json:"findingID"`
	Outcome         string `json:"outcome"`
	BeforeLevel     string `json:"beforeLevel,omitempty"`
	AfterLevel      string `json:"afterLevel,omitempty"`
	ClosureReason   string `json:"closureReason,omitempty"`
	EvidenceSummary string `json:"evidenceSummary"`
}

type TrialAssessment struct {
	AssessmentID           string            `json:"assessmentID"`
	CaseID                 string            `json:"caseID"`
	PasteFormula           string            `json:"pasteFormula"`
	WettingDuration        string            `json:"wettingDuration"`
	DryingCondition        string            `json:"dryingCondition"`
	ReversibilityGrade     string            `json:"reversibilityGrade"`
	AppearanceChange       string            `json:"appearanceChange"`
	RiskFindings           []RiskFinding     `json:"riskFindings"`
	RiskDispositions       []RiskDisposition `json:"riskDispositions,omitempty"`
	Decision               Decision          `json:"decision"`
	EvidenceDigest         string            `json:"evidenceDigest"`
	Revision               int               `json:"revision"`
	SupersedesAssessmentID string            `json:"supersedesAssessmentID,omitempty"`
	RevisionReason         string            `json:"revisionReason,omitempty"`
	RiskChanges            []RiskChange      `json:"riskChanges"`
	RecordedBy             string            `json:"recordedBy"`
	RecordedAt             time.Time         `json:"recordedAt"`
	RecordedVersion        int               `json:"recordedVersion"`
}

type EvidenceReference struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type EvidenceEvaluation struct {
	Reference EvidenceReference `json:"reference"`
	Qualified bool              `json:"qualified"`
	Result    string            `json:"result"`
}

type RemediationProgress struct {
	Measure            string               `json:"measure"`
	ExecutedBy         string               `json:"executedBy"`
	RecordedAt         time.Time            `json:"recordedAt"`
	EvidenceReferences []EvidenceReference  `json:"evidenceReferences"`
	EvidenceResults    []EvidenceEvaluation `json:"evidenceResults"`
	Closed             bool                 `json:"closed"`
	UnmetConditions    []string             `json:"unmetConditions"`
	RecordedVersion    int                  `json:"recordedVersion"`
}

type ReviewIssue struct {
	IssueID              string                `json:"issueID"`
	Item                 string                `json:"item"`
	Opinion              string                `json:"opinion"`
	Blocking             bool                  `json:"blocking"`
	Resolved             bool                  `json:"resolved"`
	Measure              string                `json:"measure,omitempty"`
	RetestReference      string                `json:"retestReference,omitempty"`
	MetricCode           string                `json:"metricCode,omitempty"`
	EvidenceRequirements []string              `json:"evidenceRequirements,omitempty"`
	ReturnedAtVersion    int                   `json:"returnedAtVersion,omitempty"`
	ReviewRound          int                   `json:"reviewRound,omitempty"`
	Remediations         []RemediationProgress `json:"remediations"`
}

type ReviewMaterial struct {
	PaperLot     PaperLot          `json:"paperLot"`
	Plan         *TestPlan         `json:"testPlan"`
	Measurements []Measurement     `json:"measurements"`
	Trial        *TrialAssessment  `json:"latestTrial"`
	RiskStatus   RiskClosureStatus `json:"riskClosureStatus"`
	Decision     Decision          `json:"decision"`
}

type ReviewDifference struct {
	AddedMeasurementIDs  []string `json:"addedMeasurementIDs"`
	TrialRevisionChanged bool     `json:"trialRevisionChanged"`
	ClosedIssueIDs       []string `json:"closedIssueIDs"`
	DecisionChanged      bool     `json:"decisionChanged"`
}

type ReviewRound struct {
	ReviewRound      int                  `json:"reviewRound"`
	SubmittedVersion int                  `json:"submittedVersion"`
	SubmissionDigest string               `json:"submissionDigest"`
	SubmittedBy      string               `json:"submittedBy"`
	SubmissionReason string               `json:"submissionReason"`
	SubmittedAt      time.Time            `json:"submittedAt"`
	Material         ReviewMaterial       `json:"material"`
	Difference       ReviewDifference     `json:"difference"`
	Status           string               `json:"status"`
	ReviewedBy       string               `json:"reviewedBy,omitempty"`
	Confirmations    []ReviewConfirmation `json:"confirmations"`
	Issues           []ReviewIssue        `json:"issues"`
	DecisionReason   string               `json:"decisionReason,omitempty"`
	DecidedAt        *time.Time           `json:"decidedAt,omitempty"`
}

type ReviewConfirmation struct {
	Item      string `json:"item"`
	Confirmed bool   `json:"confirmed"`
	Note      string `json:"note,omitempty"`
}

type AuditEntry struct {
	Action        string    `json:"action"`
	Actor         string    `json:"actor"`
	Role          Role      `json:"role"`
	Reason        string    `json:"reason"`
	At            time.Time `json:"at"`
	BeforeVersion int       `json:"beforeVersion"`
	AfterVersion  int       `json:"afterVersion"`
}
