package domain

import "time"

func (c *SuitabilityCase) AddMeasurement(m Measurement, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if c.State != StateTesting && c.State != StateRemediation {
		return NewError("invalid_state", "当前状态不允许提交测量")
	}
	for field, value := range map[string]string{"measurementID": m.MeasurementID, "metricCode": m.MetricCode, "sampleID": m.SampleID, "method": m.Method, "unit": m.Unit, "measuredBy": m.MeasuredBy} {
		if err := requireText(value, field); err != nil {
			return err
		}
	}
	rule, ok := c.rule(m.MetricCode)
	if !ok {
		return NewError("unknown_metric", "检测项目 %s 不在锁定方案中", m.MetricCode)
	}
	if m.Unit != rule.Unit {
		return NewError("invalid_unit", "%s 单位必须为 %s", m.MetricCode, rule.Unit)
	}
	if m.Value < rule.Minimum || m.Value > rule.Maximum {
		return NewError("out_of_range", "%s 数值 %.4g 超出 [%.4g, %.4g]", m.MetricCode, m.Value, rule.Minimum, rule.Maximum)
	}
	for _, existing := range c.Measurements {
		if existing.MetricCode == m.MetricCode && existing.SampleID == m.SampleID {
			return NewError("duplicate_sample", "项目与样本组合已存在")
		}
		if existing.MeasurementID == m.MeasurementID {
			return NewError("duplicate_measurement", "measurementID 已存在")
		}
	}
	m.CaseID = c.CaseID
	m.RecordedVersion = c.Version + 1
	if m.MeasuredAt.IsZero() {
		m.MeasuredAt = now
	}
	return c.change("add_measurement", RoleTester, m.MeasuredBy, "提交理化检测结果", now, func() error {
		c.Measurements = append(c.Measurements, m)
		return nil
	})
}

type BatchItemError struct {
	Index      int    `json:"index"`
	ItemNumber int    `json:"itemNumber"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (c *SuitabilityCase) AddMeasurementsBatch(items []Measurement, expected int, now time.Time) error {
	if err := c.checkVersion(expected); err != nil {
		return err
	}
	if err := c.ensureMutable(); err != nil {
		return err
	}
	if c.State != StateTesting && c.State != StateRemediation {
		return NewError("invalid_state", "当前状态不允许提交测量")
	}
	if len(items) < 1 || len(items) > 60 {
		return NewError("validation_error", "measurements 数量应为 1 至 60")
	}
	ids := map[string]bool{}
	combinations := map[string]bool{}
	for _, m := range c.Measurements {
		ids[m.MeasurementID] = true
		combinations[m.MetricCode+"\x00"+m.SampleID] = true
	}
	errors := make([]BatchItemError, 0)
	addError := func(index int, code, message string) {
		errors = append(errors, BatchItemError{Index: index, ItemNumber: index + 1, Code: code, Message: message})
	}
	for i := range items {
		m := &items[i]
		if m.MeasurementID == "" {
			addError(i, "validation_error", "measurementID 不能为空")
		}
		if m.MetricCode == "" {
			addError(i, "validation_error", "metricCode 不能为空")
		}
		if m.SampleID == "" {
			addError(i, "validation_error", "sampleID 不能为空")
		}
		if m.Method == "" {
			addError(i, "validation_error", "method 不能为空")
		}
		if m.Unit == "" {
			addError(i, "validation_error", "unit 不能为空")
		}
		if m.MeasuredBy == "" {
			addError(i, "validation_error", "measuredBy 不能为空")
		}
		rule, ok := c.rule(m.MetricCode)
		if !ok {
			addError(i, "unknown_metric", "检测项目不在锁定方案中")
		} else {
			if m.Unit != rule.Unit {
				addError(i, "invalid_unit", "单位必须为 "+rule.Unit)
			}
			if m.Value < rule.Minimum || m.Value > rule.Maximum {
				addError(i, "out_of_range", "数值超出锁定范围")
			}
		}
		if m.MeasurementID != "" {
			if ids[m.MeasurementID] {
				addError(i, "duplicate_measurement", "measurementID 已存在或在批次中重复")
			}
			ids[m.MeasurementID] = true
		}
		if m.MetricCode != "" && m.SampleID != "" {
			key := m.MetricCode + "\x00" + m.SampleID
			if combinations[key] {
				addError(i, "duplicate_sample", "项目与样本组合已存在或在批次中重复")
			}
			combinations[key] = true
		}
	}
	if len(errors) > 0 {
		return NewDetailedError("batch_validation_error", "批量测量校验失败，整批未入账", errors)
	}
	for i := range items {
		items[i].CaseID = c.CaseID
		items[i].RecordedVersion = c.Version + 1
		if items[i].MeasuredAt.IsZero() {
			items[i].MeasuredAt = now
		}
	}
	return c.change("add_measurement_batch", RoleTester, items[0].MeasuredBy, "批量提交理化检测结果", now, func() error { c.Measurements = append(c.Measurements, items...); return nil })
}

func (c *SuitabilityCase) TestingComplete() bool {
	return c.Plan != nil && len(c.MissingMeasurements()) == 0
}
