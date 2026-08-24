package httpapi

import (
	"net/http"

	"paperfit-release/internal/application"
)

func (a *API) RecordTrialHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.TrialRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.RecordTrial(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) SubmitReviewHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.SubmitReviewRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.SubmitReview(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) ReviewHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.ReviewRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.Review(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) RemediationHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.RemediationRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.Remediate(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) FreezeHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.FreezeRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.Freeze(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) IssueCredentialHandler(w http.ResponseWriter, r *http.Request) {
	ctx, err := commandContext(r)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	var request application.IssueCredentialRequest
	if err = decodeJSON(w, r, &request); err != nil {
		writeError(w, r, err, 0)
		return
	}
	result, replayed, err := a.service.IssueCredential(ctx, r.PathValue("caseID"), request)
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": result, "replayed": replayed})
}

func (a *API) GetCredentialHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.Credential(r.PathValue("credentialNumber"))
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}
func (a *API) VerifyCredentialHandler(w http.ResponseWriter, r *http.Request) {
	result := a.service.VerifyCredentialContext(r.PathValue("credentialNumber"), r.URL.Query().Get("credentialHash"), r.URL.Query().Get("inputDigest"), r.URL.Query().Get("artifactRef"), r.URL.Query().Get("repairArea"), r.URL.Query().Get("paperLotID"))
	status := http.StatusOK
	if result.Status == "not_found" {
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]any{"data": result})
}
