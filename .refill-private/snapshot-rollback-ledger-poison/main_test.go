package snapshotrollbackledgerpoison_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

func TestSnapshotFailureRetryMustNotPoisonLedger(t *testing.T) {
	directory := t.TempDir()
	store, err := eventstore.Open(directory)
	if err != nil {
		t.Fatal(err)
	}

	projectionPath := filepath.Join(directory, "projection.json")
	if err := os.Remove(projectionPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(projectionPath, 0750); err != nil {
		t.Fatal(err)
	}

	createdAt := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	caseRecord, err := domain.NewCase(
		"case-snapshot-retry",
		"PF-SNAPSHOT-RETRY",
		"artifact-snapshot-retry",
		"竹纸",
		"书口",
		domain.PaperLot{
			PaperLotID:       "lot-snapshot-retry",
			Maker:            "测试纸坊",
			Origin:           "泾县",
			FiberComposition: "楮皮",
			ProductionDate:   "2026-08-01",
			NominalWeightGSM: 30,
			SheetIdentifier:  "sheet-snapshot-retry",
		},
		"restorer-snapshot-retry",
		createdAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	commit := eventstore.CommitRequest{
		ExpectedVersion: 0,
		EventType:       "case.created",
		Case:            caseRecord,
	}
	if err := store.Commit(commit); err == nil {
		t.Fatal("投影路径被目录占用时提交应报告快照替换失败")
	}

	if err := os.Remove(projectionPath); err != nil {
		t.Fatal(err)
	}
	_ = store.Commit(commit)

	reopened, err := eventstore.Open(directory)
	if err != nil {
		t.Fatalf("快照失败后的重试不应污染已同步账本: %v", err)
	}
	if reopened.LastSequence() != 1 {
		t.Fatalf("重启后应只有一个已提交事件，实际序号为 %d", reopened.LastSequence())
	}
	if _, ok := reopened.CaseByID(caseRecord.CaseID); !ok {
		t.Fatal("重启后应恢复首次已同步的档案事件")
	}
}
