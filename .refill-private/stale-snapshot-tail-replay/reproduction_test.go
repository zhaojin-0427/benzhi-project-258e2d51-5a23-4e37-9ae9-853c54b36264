package stale_snapshot_tail_replay_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

func TestStaleSnapshotMustReplayDurableLedgerTail(t *testing.T) {
	directory := t.TempDir()
	store, err := eventstore.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	lot := domain.PaperLot{
		PaperLotID:       "lot-recovery",
		Maker:            "恢复测试纸坊",
		Origin:           "安徽泾县",
		FiberComposition: "楮皮纤维",
		ProductionDate:   "2026-01-01",
		NominalWeightGSM: 35,
		SheetIdentifier:  "sheet-recovery",
	}
	c, err := domain.NewCase("case-recovery", "PF-RECOVERY", "古籍甲", "竹纸", "书页边缘", lot, "修复师甲", now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Commit(eventstore.CommitRequest{ExpectedVersion: 0, EventType: "case.created", Case: c}); err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(directory, "projection.json")
	staleSnapshot, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.UpdateDraft("古籍甲", "竹纸", "书脊破损处", lot, "修复师甲", "补充修复部位", 1, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = store.Commit(eventstore.CommitRequest{ExpectedVersion: 1, EventType: "case.draft_updated", Case: c}); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(snapshotPath, staleSnapshot, 0640); err != nil {
		t.Fatal(err)
	}

	reopened, err := eventstore.Open(directory)
	if err != nil {
		t.Fatalf("合法的落后快照应由账本尾部恢复: %v", err)
	}
	loaded, ok := reopened.CaseByID(c.CaseID)
	if !ok {
		t.Fatal("重启后档案丢失")
	}
	if reopened.LastSequence() != 2 || loaded.Version != 2 || loaded.RepairArea != "书脊破损处" {
		t.Fatalf("已 Sync 的第 2 条事件未在重启时重放: sequence=%d version=%d repairArea=%q", reopened.LastSequence(), loaded.Version, loaded.RepairArea)
	}
}
