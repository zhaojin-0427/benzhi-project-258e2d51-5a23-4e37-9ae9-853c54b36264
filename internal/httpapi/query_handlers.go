package httpapi

import (
	"net/http"
	"strconv"

	"paperfit-release/internal/application"
	"paperfit-release/internal/domain"
)

func (a *API) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	report, err := a.service.Readiness(r.PathValue("caseID"))
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": report})
}

func (a *API) AuditTrailHandler(w http.ResponseWriter, r *http.Request) {
	query := application.AuditQuery{Action: r.URL.Query().Get("action"), Role: domain.Role(r.URL.Query().Get("role"))}
	if raw := r.URL.Query().Get("afterVersion"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			writeError(w, r, domain.NewError("validation_error", "afterVersion 必须是非负整数"), 0)
			return
		}
		query.AfterVersion = value
	}
	if query.Role != "" && query.Role != domain.RoleRestorer && query.Role != domain.RoleTester && query.Role != domain.RoleReviewer {
		writeError(w, r, domain.NewError("validation_error", "role 查询值无效"), 0)
		return
	}
	trail, err := a.service.AuditTrail(r.PathValue("caseID"), query)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": trail})
}

func (a *API) TrialHistoryHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.TrialHistory(r.PathValue("caseID"))
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "count": len(result)})
}

func (a *API) ReviewRoundsHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.ReviewRounds(r.PathValue("caseID"))
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "count": len(result)})
}

func (a *API) ReviewRoundHandler(w http.ResponseWriter, r *http.Request) {
	round, err := strconv.Atoi(r.PathValue("reviewRound"))
	if err != nil || round < 1 {
		writeError(w, r, domain.NewError("validation_error", "reviewRound 必须是正整数"), 0)
		return
	}
	result, err := a.service.ReviewRound(r.PathValue("caseID"), round)
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (a *API) ClosureMatrixHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.ClosureMatrix(r.PathValue("caseID"))
	if err != nil {
		writeError(w, r, err, 0)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result, "count": len(result)})
}

func (a *API) CredentialPrecheckHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.CredentialPrecheck(r.PathValue("caseID"))
	if err != nil {
		writeCaseError(a, w, r, err, r.PathValue("caseID"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}
