package application

import (
	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

func (s *Service) SubmitReview(ctx Context, id string, request SubmitReviewRequest) (*domain.SuitabilityCase, bool, error) {
	if err := requireRole(ctx, domain.RoleRestorer); err != nil {
		return nil, false, err
	}
	return execute(s, ctx, "submit_review:"+id, request, 200, func() (*domain.SuitabilityCase, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return nil, eventstore.CommitRequest{}, err
		}
		before := c.Version
		err = c.SubmitReview(ctx.Actor, request.Reason, request.ExpectedVersion, s.now())
		return c, eventstore.CommitRequest{ExpectedVersion: before, EventType: "review.submitted", Case: c}, err
	})
}

func (s *Service) Review(ctx Context, id string, request ReviewRequest) (*domain.SuitabilityCase, bool, error) {
	if err := requireRole(ctx, domain.RoleReviewer); err != nil {
		return nil, false, err
	}
	return execute(s, ctx, "review:"+id, request, 200, func() (*domain.SuitabilityCase, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return nil, eventstore.CommitRequest{}, err
		}
		before := c.Version
		err = c.ReviewBound(ctx.Actor, request.Approved, request.Confirmations, request.Issues, request.Reason, request.ExpectedVersion, request.ReviewRound, request.SubmissionDigest, s.now())
		typeName := "review.returned"
		if request.Approved {
			typeName = "review.approved"
		}
		return c, eventstore.CommitRequest{ExpectedVersion: before, EventType: typeName, Case: c}, err
	})
}

func (s *Service) Remediate(ctx Context, id string, request RemediationRequest) (*domain.SuitabilityCase, bool, error) {
	if err := requireRole(ctx, domain.RoleRestorer, domain.RoleTester); err != nil {
		return nil, false, err
	}
	return execute(s, ctx, "remediate:"+id+":"+request.IssueID, request, 200, func() (*domain.SuitabilityCase, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return nil, eventstore.CommitRequest{}, err
		}
		before := c.Version
		if len(request.EvidenceReferences) > 0 {
			err = c.RemediateWithEvidence(request.IssueID, request.Measure, ctx.Actor, ctx.Role, request.EvidenceReferences, request.ExpectedVersion, s.now())
		} else {
			err = c.Remediate(request.IssueID, request.Measure, request.RetestReference, ctx.Actor, request.ExpectedVersion, s.now())
		}
		return c, eventstore.CommitRequest{ExpectedVersion: before, EventType: "issue.remediated", Case: c}, err
	})
}

func (s *Service) Freeze(ctx Context, id string, request FreezeRequest) (*domain.SuitabilityCase, bool, error) {
	if err := requireRole(ctx, domain.RoleReviewer); err != nil {
		return nil, false, err
	}
	return execute(s, ctx, "freeze:"+id, request, 200, func() (*domain.SuitabilityCase, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return nil, eventstore.CommitRequest{}, err
		}
		before := c.Version
		err = c.Freeze(ctx.Actor, request.Reason, request.ExpectedVersion, s.now())
		return c, eventstore.CommitRequest{ExpectedVersion: before, EventType: "case.frozen", Case: c}, err
	})
}

type CredentialResult struct {
	Credential  domain.ReleaseCredential `json:"credential"`
	CaseVersion int                      `json:"caseVersion"`
}

func (s *Service) IssueCredential(ctx Context, id string, request IssueCredentialRequest) (CredentialResult, bool, error) {
	if err := requireRole(ctx, domain.RoleReviewer); err != nil {
		return CredentialResult{}, false, err
	}
	return execute(s, ctx, "issue_credential:"+id, request, 201, func() (CredentialResult, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return CredentialResult{}, eventstore.CommitRequest{}, err
		}
		before := c.Version
		credential, err := c.IssueCredentialScoped(randomID("PFC"), request.UsageScope, request.ConfirmedRestrictionIDs, ctx.Actor, request.ExpectedVersion, s.now())
		result := CredentialResult{Credential: credential, CaseVersion: c.Version}
		return result, eventstore.CommitRequest{ExpectedVersion: before, EventType: "credential.issued", Case: c, Credential: &credential}, err
	})
}
