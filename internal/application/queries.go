package application

import (
	"paperfit-release/internal/domain"
)

type CaseView struct {
	Case                *domain.SuitabilityCase  `json:"case"`
	MissingMeasurements []string                 `json:"missingMeasurements"`
	DetectionProgress   []domain.MetricProgress  `json:"detectionProgress"`
	OpenBlockingIssues  []domain.ReviewIssue     `json:"openBlockingIssues"`
	RiskStatus          domain.RiskClosureStatus `json:"riskStatus"`
}

func (s *Service) Case(id string) (CaseView, error) {
	c, err := s.load(id)
	if err != nil {
		return CaseView{}, err
	}
	return CaseView{Case: c, MissingMeasurements: c.MissingMeasurements(), DetectionProgress: c.DetectionProgress(), OpenBlockingIssues: c.OpenBlockingIssues(), RiskStatus: c.RiskStatus()}, nil
}

func (s *Service) FindCases(caseNumber, paperLotID string) []*domain.SuitabilityCase {
	if caseNumber != "" {
		if c, ok := s.store.CaseByNumber(caseNumber); ok {
			return []*domain.SuitabilityCase{c}
		}
		return []*domain.SuitabilityCase{}
	}
	if paperLotID != "" {
		return s.store.CasesByPaperLot(paperLotID)
	}
	return s.store.ListCases()
}

type VerificationResult struct {
	CredentialNumber string                    `json:"credentialNumber"`
	Valid            bool                      `json:"valid"`
	Status           string                    `json:"status"`
	Credential       *domain.ReleaseCredential `json:"credential,omitempty"`
	CredentialStatus string                    `json:"credentialStatus"`
	ScopeStatus      string                    `json:"scopeStatus"`
	ScopeMismatches  []string                  `json:"scopeMismatches"`
}

func (s *Service) Credential(number string) (domain.ReleaseCredential, error) {
	credential, ok := s.store.Credential(number)
	if !ok {
		return domain.ReleaseCredential{}, domain.NewError("not_found", "放行凭据不存在")
	}
	return credential, nil
}

func (s *Service) VerifyCredential(number string) VerificationResult {
	credential, ok := s.store.Credential(number)
	if !ok {
		return VerificationResult{CredentialNumber: number, Valid: false, Status: "not_found", CredentialStatus: "not_found", ScopeStatus: "not_checked", ScopeMismatches: []string{}}
	}
	c, caseFound := s.store.CaseByID(credential.CaseID)
	if !caseFound {
		c = nil
	}
	valid, status := domain.VerifyCredential(credential, c)
	return VerificationResult{CredentialNumber: number, Valid: valid, Status: status, CredentialStatus: status, ScopeStatus: "not_checked", ScopeMismatches: []string{}, Credential: &credential}
}

func (s *Service) VerifyCredentialContext(number, presentedHash, presentedInputDigest, artifactRef, repairArea, paperLotID string) VerificationResult {
	result := s.VerifyCredentialEvidence(number, presentedHash, presentedInputDigest)
	result.CredentialStatus = result.Status
	if result.Credential == nil || !result.Valid {
		result.ScopeStatus = "not_checked"
		return result
	}
	result.ScopeStatus, result.ScopeMismatches = domain.VerifyCredentialScope(*result.Credential, artifactRef, repairArea, paperLotID)
	return result
}

func (s *Service) TrialHistory(id string) ([]domain.TrialAssessment, error) {
	c, err := s.load(id)
	if err != nil {
		return nil, err
	}
	return append([]domain.TrialAssessment(nil), c.TrialHistory...), nil
}

func (s *Service) ReviewRounds(id string) ([]domain.ReviewRound, error) {
	c, err := s.load(id)
	if err != nil {
		return nil, err
	}
	return append([]domain.ReviewRound(nil), c.ReviewRounds...), nil
}

func (s *Service) ReviewRound(id string, round int) (domain.ReviewRound, error) {
	rounds, err := s.ReviewRounds(id)
	if err != nil {
		return domain.ReviewRound{}, err
	}
	for _, item := range rounds {
		if item.ReviewRound == round {
			return item, nil
		}
	}
	return domain.ReviewRound{}, domain.NewError("not_found", "评审轮次不存在")
}

func (s *Service) ClosureMatrix(id string) ([]domain.ClosureMatrixRow, error) {
	c, err := s.load(id)
	if err != nil {
		return nil, err
	}
	return c.ClosureMatrix(), nil
}

func (s *Service) CredentialPrecheck(id string) (domain.CredentialPrecheck, error) {
	c, err := s.load(id)
	if err != nil {
		return domain.CredentialPrecheck{}, err
	}
	return c.CredentialPrecheck()
}

func (s *Service) VerifyCredentialEvidence(number, presentedHash, presentedInputDigest string) VerificationResult {
	result := s.VerifyCredential(number)
	if !result.Valid || result.Credential == nil {
		return result
	}
	if presentedHash != "" && presentedHash != result.Credential.CredentialHash {
		result.Valid = false
		result.Status = "credential_content_mismatch"
		return result
	}
	if presentedInputDigest != "" && presentedInputDigest != result.Credential.InputDigest {
		result.Valid = false
		result.Status = "frozen_content_mismatch"
	}
	return result
}

func (s *Service) Status() map[string]any {
	stats := s.store.Statistics()
	return map[string]any{"status": "ok", "schemaVersion": stats.SchemaVersion, "eventSequence": stats.LastSequence, "caseCount": stats.CaseCount, "credentialCount": stats.CredentialCount}
}
