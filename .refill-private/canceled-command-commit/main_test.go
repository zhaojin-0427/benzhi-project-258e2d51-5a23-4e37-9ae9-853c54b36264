package canceledcommandcommit_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"paperfit-release/internal/application"
	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
	"paperfit-release/internal/httpapi"
)

func TestCanceledCommandMustNotCommit(t *testing.T) {
	t.Run("canceled-before-action", func(t *testing.T) {
		store, err := eventstore.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		clockCalls := 0
		service := application.NewServiceWithClock(store, func() time.Time {
			clockCalls++
			return time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
		})
		api := httpapi.New(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
		body := `{
		"caseNumber":"PF-CANCELED",
		"artifactRef":"古籍-取消测试",
		"artifactMaterial":"竹纸",
		"repairArea":"书脊",
		"paperLot":{
			"paperLotID":"lot-canceled",
			"maker":"纸坊",
			"origin":"泾县",
			"fiberComposition":"楮皮",
			"productionDate":"2026-08-25",
			"nominalWeightGSM":35,
			"sheetIdentifier":"sheet-canceled"
		}
	}`

		requestContext, cancel := context.WithCancel(context.Background())
		cancel()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/suitability-cases", strings.NewReader(body)).WithContext(requestContext)
		request.Header.Set("X-Actor", "修复师-取消测试")
		request.Header.Set("X-Role", "restorer")
		request.Header.Set("Idempotency-Key", "cancel-before-create")
		response := httptest.NewRecorder()

		api.ServeHTTP(response, request)

		if response.Code < http.StatusBadRequest {
			t.Fatalf("取消请求不应返回成功，实际状态码为 %d", response.Code)
		}
		stats := store.Statistics()
		if clockCalls != 0 || stats.CaseCount != 0 || stats.LastSequence != 0 || stats.IdempotencyCount != 0 {
			t.Fatalf("预取消请求仍启动业务 action 或写入投影和账本: clockCalls=%d stats=%#v", clockCalls, stats)
		}
	})

	t.Run("canceled-during-action", func(t *testing.T) {
		store, err := eventstore.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		clockEntered := make(chan struct{})
		continueAction := make(chan struct{})
		service := application.NewServiceWithClock(store, func() time.Time {
			close(clockEntered)
			<-continueAction
			return time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
		})
		requestContext, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			_, _, createErr := service.CreateCase(
				application.Context{Actor: "修复师-取消测试", Role: domain.RoleRestorer, IdempotencyKey: "cancel-during-create", RequestContext: requestContext},
				application.CreateCaseRequest{
					CaseNumber:       "PF-CANCEL-DURING",
					ArtifactRef:      "古籍-执行中取消测试",
					ArtifactMaterial: "竹纸",
					RepairArea:       "书脊",
					PaperLot: domain.PaperLot{
						PaperLotID:       "lot-cancel-during",
						Maker:            "纸坊",
						Origin:           "泾县",
						FiberComposition: "楮皮",
						ProductionDate:   "2026-08-25",
						NominalWeightGSM: 35,
						SheetIdentifier:  "sheet-cancel-during",
					},
				},
			)
			result <- createErr
		}()

		<-clockEntered
		cancel()
		close(continueAction)
		if createErr := <-result; !errors.Is(createErr, context.Canceled) {
			t.Fatalf("执行中取消应返回 context.Canceled，实际为 %v", createErr)
		}
		stats := store.Statistics()
		if stats.CaseCount != 0 || stats.LastSequence != 0 || stats.IdempotencyCount != 0 {
			t.Fatalf("执行中取消后仍写入投影和账本: %#v", stats)
		}
	})
}
