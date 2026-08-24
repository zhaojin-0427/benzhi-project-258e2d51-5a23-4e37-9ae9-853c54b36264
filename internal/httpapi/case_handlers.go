package httpapi

import (
	"net/http"
	"strconv"

	"paperfit-release/internal/application"
	"paperfit-release/internal/domain"
)

func (a *API) PreviewPlanHandler(w http.ResponseWriter, r *http.Request) {
	sampleCount, err := strconv.Atoi(r.URL.Query().Get("sampleCount"))
	if err != nil {
		writeError(w, r, domain.NewError("validation_error", "sampleCount 必须是整数"), 0)
		return
	}
	result, err := a.service.PreviewPlan(r.PathValue("caseID"), sampleCount)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (a *API) HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.service.Status())
}

func (a *API) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.CreateCaseRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.CreateCase(ctx, request)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) ListCasesHandler(w http.ResponseWriter, r *http.Request) {
	result := a.service.FindCases(r.URL.Query().Get("caseNumber"), r.URL.Query().Get("paperLotID"))
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "count": len(result)})
}

func (a *API) UpdateDraftHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.UpdateDraftRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.UpdateDraft(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) GetCaseHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.Case(r.PathValue("caseID"))
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (a *API) LockPlanHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.LockPlanRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.LockPlan(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) AddMeasurementHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.MeasurementRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.AddMeasurement(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) AddMeasurementsBatchHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.MeasurementBatchRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.AddMeasurementsBatch(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "replayed": replayed})
}

func writeCaseError(a *API, w http.ResponseWriter, r *http.Request, err error, id string) {
	current := 0
	if view, loadErr := a.service.Case(id); loadErr == nil {
		current = view.Case.Version
	}
	writeError(w, r, err, current)
}
