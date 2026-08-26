package concurrent_hash_workspace_test

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	"paperfit-release/internal/application"
	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

func TestConcurrentIdempotencyHashingMustIsolateWorkspace(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(previousProcs) })

	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store)

	const workers = 8
	requests := make([]application.CreateCaseRequest, workers)
	contexts := make([]application.Context, workers)
	largeDistinctText := strings.Repeat("古籍修复并发指纹隔离", 8192)
	for i := 0; i < workers; i++ {
		marker := fmt.Sprintf("-%d", i)
		requests[i] = application.CreateCaseRequest{
			CaseNumber:       "PF-HASH" + marker,
			ArtifactRef:      largeDistinctText + marker,
			ArtifactMaterial: "竹纸",
			RepairArea:       "书页边缘",
			PaperLot: domain.PaperLot{
				PaperLotID:       "LOT-HASH" + marker,
				Maker:            "纸坊",
				Origin:           "泾县",
				FiberComposition: "楮皮",
				ProductionDate:   "2026-01-01",
				NominalWeightGSM: 35,
				SheetIdentifier:  "SHEET" + marker,
			},
		}
		contexts[i] = application.Context{Actor: "修复师", Role: domain.RoleRestorer, IdempotencyKey: "hash-key" + marker}
	}

	ready := sync.WaitGroup{}
	done := sync.WaitGroup{}
	ready.Add(workers)
	done.Add(workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func(index int) {
			defer done.Done()
			ready.Done()
			<-start
			if _, _, createErr := service.CreateCase(contexts[index], requests[index]); createErr != nil {
				t.Errorf("并发创建 %d 失败: %v", index, createErr)
			}
		}(i)
	}
	ready.Wait()
	close(start)
	done.Wait()

	for i := 0; i < workers; i++ {
		_, replayed, replayErr := service.CreateCase(contexts[i], requests[i])
		if replayErr != nil || !replayed {
			t.Fatalf("相同请求的幂等重放 %d 不稳定: replayed=%v err=%v", i, replayed, replayErr)
		}
	}
}
