package domain

import (
	"sort"
	"time"
)

func (c *SuitabilityCase) reviewMaterial() ReviewMaterial {
	material := ReviewMaterial{PaperLot: c.PaperLot, Plan: c.Plan, Measurements: append([]Measurement(nil), c.Measurements...), Trial: c.Trial, RiskStatus: c.RiskStatus()}
	if c.Trial != nil {
		material.Decision = c.Trial.Decision
	}
	return material
}

func (c *SuitabilityCase) newReviewRound(actor, reason string, now time.Time) ReviewRound {
	material := c.reviewMaterial()
	round := ReviewRound{ReviewRound: len(c.ReviewRounds) + 1, SubmittedVersion: c.Version + 1, SubmittedBy: actor, SubmissionReason: reason, SubmittedAt: now, Material: material, Status: "pending", Confirmations: []ReviewConfirmation{}, Issues: []ReviewIssue{}}
	round.SubmissionDigest = HashJSON(material)
	if len(c.ReviewRounds) > 0 {
		previousRound := c.ReviewRounds[len(c.ReviewRounds)-1]
		previous := previousRound.Material
		oldMeasurements := map[string]bool{}
		for _, m := range previous.Measurements {
			oldMeasurements[m.MeasurementID] = true
		}
		for _, m := range material.Measurements {
			if !oldMeasurements[m.MeasurementID] {
				round.Difference.AddedMeasurementIDs = append(round.Difference.AddedMeasurementIDs, m.MeasurementID)
			}
		}
		oldRevision, newRevision := 0, 0
		if previous.Trial != nil {
			oldRevision = previous.Trial.Revision
		}
		if material.Trial != nil {
			newRevision = material.Trial.Revision
		}
		round.Difference.TrialRevisionChanged = oldRevision != newRevision
		oldIssues := map[string]bool{}
		for _, issue := range c.ReviewIssues {
			if issue.ReviewRound < round.ReviewRound && issue.Resolved {
				for _, progress := range issue.Remediations {
					if progress.Closed && progress.RecordedVersion > previousRound.SubmittedVersion {
						oldIssues[issue.IssueID] = true
					}
				}
			}
		}
		for id := range oldIssues {
			round.Difference.ClosedIssueIDs = append(round.Difference.ClosedIssueIDs, id)
		}
		sort.Strings(round.Difference.ClosedIssueIDs)
		round.Difference.DecisionChanged = previous.Decision != material.Decision
	}
	return round
}

func (c *SuitabilityCase) finishReviewRound(status, actor, reason string, confirmations []ReviewConfirmation, issues []ReviewIssue, now time.Time) {
	if len(c.ReviewRounds) == 0 {
		return
	}
	round := &c.ReviewRounds[len(c.ReviewRounds)-1]
	round.Status, round.ReviewedBy, round.DecisionReason = status, actor, reason
	round.Confirmations = append([]ReviewConfirmation(nil), confirmations...)
	round.Issues = append([]ReviewIssue(nil), issues...)
	round.DecidedAt = &now
}
