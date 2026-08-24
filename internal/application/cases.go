package application

import (
	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

func (s *Service) CreateCase(ctx Context, request CreateCaseRequest) (*domain.SuitabilityCase, bool, error) {
	if err := requireRole(ctx, domain.RoleRestorer); err != nil {
		return nil, false, err
	}
	return execute(s, ctx, "create_case", request, 201, func() (*domain.SuitabilityCase, eventstore.CommitRequest, error) {
		id := randomID("case")
		number := request.CaseNumber
		if number == "" {
			number = "PF-" + s.now().Format("20060102") + "-" + randomID("")[1:9]
		}
		c, err := domain.NewCase(id, number, request.ArtifactRef, request.ArtifactMaterial, request.RepairArea, request.PaperLot, ctx.Actor, s.now())
		return c, eventstore.CommitRequest{ExpectedVersion: 0, EventType: "case.created", Case: c}, err
	})
}

func (s *Service) UpdateDraft(ctx Context, id string, request UpdateDraftRequest) (*domain.SuitabilityCase, bool, error) {
	if err := requireRole(ctx, domain.RoleRestorer); err != nil {
		return nil, false, err
	}
	return execute(s, ctx, "update_draft:"+id, request, 200, func() (*domain.SuitabilityCase, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return nil, eventstore.CommitRequest{}, err
		}
		before := c.Version
		err = c.UpdateDraft(request.ArtifactRef, request.ArtifactMaterial, request.RepairArea, request.PaperLot, ctx.Actor, request.Reason, request.ExpectedVersion, s.now())
		return c, eventstore.CommitRequest{ExpectedVersion: before, EventType: "case.draft_updated", Case: c}, err
	})
}

func (s *Service) LockPlan(ctx Context, id string, request LockPlanRequest) (*domain.SuitabilityCase, bool, error) {
	if err := requireRole(ctx, domain.RoleTester); err != nil {
		return nil, false, err
	}
	return execute(s, ctx, "lock_plan:"+id, request, 200, func() (*domain.SuitabilityCase, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return nil, eventstore.CommitRequest{}, err
		}
		before := c.Version
		err = c.LockPlanWithPreview(randomID("plan"), ctx.Actor, request.SampleCount, request.ExpectedVersion, request.PreviewDigest, s.now())
		return c, eventstore.CommitRequest{ExpectedVersion: before, EventType: "plan.locked", Case: c}, err
	})
}

func (s *Service) PreviewPlan(id string, sampleCount int) (domain.PlanPreview, error) {
	c, err := s.load(id)
	if err != nil {
		return domain.PlanPreview{}, err
	}
	return domain.PreviewPlan(c, sampleCount)
}

func (s *Service) AddMeasurement(ctx Context, id string, request MeasurementRequest) (*domain.SuitabilityCase, bool, error) {
	if err := requireRole(ctx, domain.RoleTester); err != nil {
		return nil, false, err
	}
	return execute(s, ctx, "add_measurement:"+id, request, 200, func() (*domain.SuitabilityCase, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return nil, eventstore.CommitRequest{}, err
		}
		before := c.Version
		measurementID := request.MeasurementID
		if measurementID == "" {
			measurementID = randomID("measurement")
		}
		m := domain.Measurement{MeasurementID: measurementID, MetricCode: request.MetricCode, SampleID: request.SampleID, Method: request.Method, Value: request.Value, Unit: request.Unit, MeasuredBy: ctx.Actor, MeasuredAt: s.now()}
		err = c.AddMeasurement(m, request.ExpectedVersion, s.now())
		return c, eventstore.CommitRequest{ExpectedVersion: before, EventType: "measurement.added", Case: c}, err
	})
}

type MeasurementBatchResult struct {
	CaseID            string                  `json:"caseID"`
	CaseVersion       int                     `json:"caseVersion"`
	MeasurementIDs    []string                `json:"measurementIDs"`
	DetectionProgress []domain.MetricProgress `json:"detectionProgress"`
}

func (s *Service) AddMeasurementsBatch(ctx Context, id string, request MeasurementBatchRequest) (MeasurementBatchResult, bool, error) {
	if err := requireRole(ctx, domain.RoleTester); err != nil {
		return MeasurementBatchResult{}, false, err
	}
	return execute(s, ctx, "add_measurement_batch:"+id, request, 200, func() (MeasurementBatchResult, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return MeasurementBatchResult{}, eventstore.CommitRequest{}, err
		}
		before, now := c.Version, s.now()
		items := make([]domain.Measurement, len(request.Measurements))
		ids := make([]string, len(items))
		for i, input := range request.Measurements {
			id := input.MeasurementID
			if id == "" {
				id = randomID("measurement")
			}
			measuredBy := ctx.Actor
			if input.MeasuredBy != "" && input.MeasuredBy != ctx.Actor {
				return MeasurementBatchResult{}, eventstore.CommitRequest{}, domain.NewDetailedError("batch_validation_error", "批量测量校验失败，整批未入账", []domain.BatchItemError{{Index: i, ItemNumber: i + 1, Code: "tester_mismatch", Message: "measuredBy 必须为当前检测员"}})
			}
			items[i] = domain.Measurement{MeasurementID: id, MetricCode: input.MetricCode, SampleID: input.SampleID, Method: input.Method, Value: input.Value, Unit: input.Unit, MeasuredBy: measuredBy, MeasuredAt: now}
			ids[i] = id
		}
		err = c.AddMeasurementsBatch(items, request.ExpectedVersion, now)
		result := MeasurementBatchResult{CaseID: c.CaseID, CaseVersion: c.Version, MeasurementIDs: ids, DetectionProgress: c.DetectionProgress()}
		return result, eventstore.CommitRequest{ExpectedVersion: before, EventType: "measurement.batch_added", Case: c}, err
	})
}

func (s *Service) RecordTrial(ctx Context, id string, request TrialRequest) (*domain.SuitabilityCase, bool, error) {
	if err := requireRole(ctx, domain.RoleRestorer); err != nil {
		return nil, false, err
	}
	return execute(s, ctx, "record_trial:"+id, request, 200, func() (*domain.SuitabilityCase, eventstore.CommitRequest, error) {
		c, err := s.load(id)
		if err != nil {
			return nil, eventstore.CommitRequest{}, err
		}
		before := c.Version
		trial := domain.TrialAssessment{AssessmentID: request.AssessmentID, PasteFormula: request.PasteFormula, WettingDuration: request.WettingDuration, DryingCondition: request.DryingCondition, ReversibilityGrade: request.ReversibilityGrade, AppearanceChange: request.AppearanceChange, RiskFindings: request.RiskFindings, RiskDispositions: request.RiskDispositions, Decision: request.Decision, EvidenceDigest: request.EvidenceDigest, SupersedesAssessmentID: request.SupersedesAssessmentID, RevisionReason: request.RevisionReason}
		if trial.AssessmentID == "" {
			trial.AssessmentID = randomID("trial")
		}
		eventType := "trial.recorded"
		if c.Trial != nil {
			eventType = "trial.revised"
		}
		err = c.RecordTrial(trial, ctx.Actor, request.ExpectedVersion, s.now())
		return c, eventstore.CommitRequest{ExpectedVersion: before, EventType: eventType, Case: c}, err
	})
}
