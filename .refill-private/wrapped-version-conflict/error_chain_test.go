package wrapped_version_conflict_test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"paperfit-release/internal/application"
	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
	"paperfit-release/internal/httpapi"
)

func TestWrappedConcurrentVersionConflictKeepsHTTPStatus(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	initial, err := domain.NewCase(
		"case-conflict",
		"PF-CONFLICT",
		"古籍甲",
		"竹纸",
		"书脊",
		domain.PaperLot{
			PaperLotID:       "lot-initial",
			Maker:            "纸坊",
			Origin:           "泾县",
			FiberComposition: "楮皮",
			ProductionDate:   "2026-01-01",
			NominalWeightGSM: 35,
			SheetIdentifier:  "sheet-initial",
		},
		"修复师",
		fixed,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Commit(eventstore.CommitRequest{ExpectedVersion: 0, EventType: "case.created", Case: initial}); err != nil {
		t.Fatal(err)
	}

	ready := make(chan struct{})
	release := make(chan struct{})
	var arrivalMu sync.Mutex
	arrivals := 0
	clock := func() time.Time {
		arrivalMu.Lock()
		arrivals++
		if arrivals == 2 {
			close(ready)
		}
		arrivalMu.Unlock()
		<-release
		return fixed.Add(time.Minute)
	}
	service := application.NewServiceWithClock(store, clock)
	api := httpapi.New(service, slog.New(slog.NewTextHandler(io.Discard, nil)))

	type outcome struct {
		status int
		body   string
	}
	outcomes := make(chan outcome, 2)
	var requests sync.WaitGroup
	for i, variant := range []string{"a", "b"} {
		requests.Add(1)
		go func(i int, variant string) {
			defer requests.Done()
			body := []byte(`{"expectedVersion":1,"artifactRef":"古籍` + variant + `","artifactMaterial":"竹纸","repairArea":"书脊","paperLot":{"paperLotID":"lot-` + variant + `","maker":"纸坊","origin":"泾县","fiberComposition":"楮皮","productionDate":"2026-01-01","nominalWeightGSM":35,"sheetIdentifier":"sheet-` + variant + `"},"reason":"并发修订"}`)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/suitability-cases/case-conflict/draft", bytes.NewReader(body))
			req.Header.Set("X-Actor", "修复师")
			req.Header.Set("X-Role", "restorer")
			req.Header.Set("Idempotency-Key", "conflict-"+variant)
			recorder := httptest.NewRecorder()
			api.ServeHTTP(recorder, req)
			outcomes <- outcome{status: recorder.Code, body: recorder.Body.String()}
		}(i, variant)
	}

	<-ready
	close(release)
	requests.Wait()
	close(outcomes)

	statuses := make([]int, 0, 2)
	bodies := make([]string, 0, 2)
	for result := range outcomes {
		statuses = append(statuses, result.status)
		bodies = append(bodies, result.body)
	}
	sort.Ints(statuses)
	if len(statuses) != 2 || statuses[0] != http.StatusOK || statuses[1] != http.StatusConflict {
		t.Fatalf("并发版本冲突应映射为 HTTP 409，实际状态=%v，响应=%q", statuses, bodies)
	}
}
