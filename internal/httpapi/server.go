package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"paperfit-release/internal/application"
)

type API struct {
	service *application.Service
	logger  *slog.Logger
	mux     *http.ServeMux
}

func New(service *application.Service, logger *slog.Logger) *API {
	if logger == nil {
		logger = slog.Default()
	}
	a := &API{service: service, logger: logger, mux: http.NewServeMux()}
	a.routes()
	return a
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /healthz", a.HealthHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases", a.CreateCaseHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases", a.ListCasesHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases/{caseID}", a.GetCaseHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases/{caseID}/readiness", a.ReadinessHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases/{caseID}/audit", a.AuditTrailHandler)
	a.mux.HandleFunc("PUT /api/v1/suitability-cases/{caseID}/draft", a.UpdateDraftHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases/{caseID}/test-plan", a.LockPlanHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases/{caseID}/test-plan/preview", a.PreviewPlanHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases/{caseID}/measurements", a.AddMeasurementHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases/{caseID}/measurements/batch", a.AddMeasurementsBatchHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases/{caseID}/trial-assessment", a.RecordTrialHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases/{caseID}/trial-assessments", a.TrialHistoryHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases/{caseID}/review-submissions", a.SubmitReviewHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases/{caseID}/reviews", a.ReviewHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases/{caseID}/review-rounds", a.ReviewRoundsHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases/{caseID}/review-rounds/{reviewRound}", a.ReviewRoundHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases/{caseID}/remediations", a.RemediationHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases/{caseID}/closure-matrix", a.ClosureMatrixHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases/{caseID}/freeze", a.FreezeHandler)
	a.mux.HandleFunc("POST /api/v1/suitability-cases/{caseID}/credentials", a.IssueCredentialHandler)
	a.mux.HandleFunc("GET /api/v1/suitability-cases/{caseID}/credentials/precheck", a.CredentialPrecheckHandler)
	a.mux.HandleFunc("GET /api/v1/credentials/{credentialNumber}", a.GetCredentialHandler)
	a.mux.HandleFunc("GET /api/v1/credentials/{credentialNumber}/verify", a.VerifyCredentialHandler)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
func (r *statusRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	r = withRequestID(r)
	w.Header().Set("X-Request-ID", requestIDFrom(r))
	rec := &statusRecorder{ResponseWriter: w}
	a.mux.ServeHTTP(rec, r)
	a.logger.Info("http_access", "request_id", requestIDFrom(r), "method", r.Method, "path", r.URL.Path, "status", rec.status, "bytes", rec.bytes, "duration_ms", time.Since(started).Milliseconds())
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
}
