package rotatedledger_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

func TestRotatedLedgerMustRemainRestartRecoverable(t *testing.T) {
	directory := t.TempDir()
	store, err := eventstore.Open(directory)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	first := newCase(t, "case-before-rotation", "PF-ROTATE-001", "LOT-ROTATE-001")
	if err := store.Commit(eventstore.CommitRequest{ExpectedVersion: 0, EventType: "case.created", Case: first}); err != nil {
		t.Fatalf("commit before rotation: %v", err)
	}

	ledgerPath := filepath.Join(directory, "events.jsonl")
	if err := os.Rename(ledgerPath, filepath.Join(directory, "events.jsonl.1")); err != nil {
		t.Fatalf("rotate ledger: %v", err)
	}
	replacement, err := os.OpenFile(ledgerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0640)
	if err != nil {
		t.Fatalf("create replacement ledger: %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatalf("close replacement ledger: %v", err)
	}

	second := newCase(t, "case-after-rotation", "PF-ROTATE-002", "LOT-ROTATE-002")
	if err := store.Commit(eventstore.CommitRequest{ExpectedVersion: 0, EventType: "case.created", Case: second}); err != nil {
		t.Fatalf("commit after rotation: %v", err)
	}
	if _, ok := store.CaseByID(second.CaseID); !ok {
		t.Fatal("successful commit was not visible before restart")
	}

	restarted, err := eventstore.Open(directory)
	if err != nil {
		t.Fatalf("restart store: %v", err)
	}
	if _, ok := restarted.CaseByID(second.CaseID); !ok {
		t.Fatalf("successful post-rotation commit disappeared after restart: %s", second.CaseID)
	}
}

func newCase(t *testing.T, id, number, lotID string) *domain.SuitabilityCase {
	t.Helper()
	created, err := domain.NewCase(id, number, "古籍-轮转复现", "竹纸", "书页边缘", domain.PaperLot{
		PaperLotID:       lotID,
		Maker:            "复现纸坊",
		Origin:           "安徽泾县",
		FiberComposition: "楮皮纤维",
		ProductionDate:   "2026-08-25",
		NominalWeightGSM: 35,
		SheetIdentifier:  "SHEET-" + lotID,
	}, "复现修复师", time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("new case: %v", err)
	}
	return created
}
