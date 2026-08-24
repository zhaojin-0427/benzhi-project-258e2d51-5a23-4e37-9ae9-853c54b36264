package application

import "paperfit-release/internal/domain"

type Context struct {
	Actor          string
	Role           domain.Role
	IdempotencyKey string
}

type CreateCaseRequest struct {
	CaseNumber       string          `json:"caseNumber"`
	ArtifactRef      string          `json:"artifactRef"`
	ArtifactMaterial string          `json:"artifactMaterial"`
	RepairArea       string          `json:"repairArea"`
	PaperLot         domain.PaperLot `json:"paperLot"`
}

type UpdateDraftRequest struct {
	ExpectedVersion  int             `json:"expectedVersion"`
	ArtifactRef      string          `json:"artifactRef"`
	ArtifactMaterial string          `json:"artifactMaterial"`
	RepairArea       string          `json:"repairArea"`
	PaperLot         domain.PaperLot `json:"paperLot"`
	Reason           string          `json:"reason"`
}

type VersionedRequest struct {
	ExpectedVersion int `json:"expectedVersion"`
}

type LockPlanRequest struct {
	ExpectedVersion int    `json:"expectedVersion"`
	SampleCount     int    `json:"sampleCount"`
	PreviewDigest   string `json:"previewDigest"`
}

type MeasurementItemRequest struct {
	MeasurementID string  `json:"measurementID"`
	MetricCode    string  `json:"metricCode"`
	SampleID      string  `json:"sampleID"`
	Method        string  `json:"method"`
	Value         float64 `json:"value"`
	Unit          string  `json:"unit"`
	MeasuredBy    string  `json:"measuredBy,omitempty"`
}

type MeasurementBatchRequest struct {
	ExpectedVersion int                      `json:"expectedVersion"`
	Measurements    []MeasurementItemRequest `json:"measurements"`
}

type MeasurementRequest struct {
	ExpectedVersion int     `json:"expectedVersion"`
	MeasurementID   string  `json:"measurementID"`
	MetricCode      string  `json:"metricCode"`
	SampleID        string  `json:"sampleID"`
	Method          string  `json:"method"`
	Value           float64 `json:"value"`
	Unit            string  `json:"unit"`
}

type TrialRequest struct {
	ExpectedVersion        int                      `json:"expectedVersion"`
	AssessmentID           string                   `json:"assessmentID"`
	PasteFormula           string                   `json:"pasteFormula"`
	WettingDuration        string                   `json:"wettingDuration"`
	DryingCondition        string                   `json:"dryingCondition"`
	ReversibilityGrade     string                   `json:"reversibilityGrade"`
	AppearanceChange       string                   `json:"appearanceChange"`
	RiskFindings           []domain.RiskFinding     `json:"riskFindings"`
	Decision               domain.Decision          `json:"decision"`
	EvidenceDigest         string                   `json:"evidenceDigest"`
	SupersedesAssessmentID string                   `json:"supersedesAssessmentID"`
	RevisionReason         string                   `json:"revisionReason"`
	RiskDispositions       []domain.RiskDisposition `json:"riskDispositions"`
}

type SubmitReviewRequest struct {
	ExpectedVersion int    `json:"expectedVersion"`
	Reason          string `json:"reason"`
}

type ReviewRequest struct {
	ExpectedVersion  int                         `json:"expectedVersion"`
	Approved         bool                        `json:"approved"`
	Reason           string                      `json:"reason"`
	Confirmations    []domain.ReviewConfirmation `json:"confirmations"`
	Issues           []domain.ReviewIssue        `json:"issues"`
	ReviewRound      int                         `json:"reviewRound"`
	SubmissionDigest string                      `json:"submissionDigest"`
}

type RemediationRequest struct {
	ExpectedVersion    int                        `json:"expectedVersion"`
	IssueID            string                     `json:"issueID"`
	Measure            string                     `json:"measure"`
	RetestReference    string                     `json:"retestReference"`
	EvidenceReferences []domain.EvidenceReference `json:"evidenceReferences"`
}

type FreezeRequest struct {
	ExpectedVersion int    `json:"expectedVersion"`
	Reason          string `json:"reason"`
}

type IssueCredentialRequest struct {
	ExpectedVersion         int               `json:"expectedVersion"`
	UsageScope              domain.UsageScope `json:"usageScope"`
	ConfirmedRestrictionIDs []string          `json:"confirmedRestrictionIDs"`
}
