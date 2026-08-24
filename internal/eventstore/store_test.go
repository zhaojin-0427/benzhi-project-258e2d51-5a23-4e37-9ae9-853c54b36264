package eventstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"paperfit-release/internal/domain"
)

func testCase(t *testing.T) *domain.SuitabilityCase {
	t.Helper()
	c, err := domain.NewCase("case-1", "PF-1", "古籍", "竹纸", "边缘", domain.PaperLot{PaperLotID: "lot-1", Maker: "纸坊", Origin: "泾县", FiberComposition: "楮皮", ProductionDate: "2026-01-01", NominalWeightGSM: 30, SheetIdentifier: "S1"}, "修复师", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestReplayRestoresProjectionAndIdempotency(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	c := testCase(t)
	record := &IdempotencyRecord{Key: "key-1", Operation: "create", Fingerprint: "fp", Status: 201, Response: []byte(`{"caseID":"case-1"}`)}
	if err = store.Commit(CommitRequest{ExpectedVersion: 0, EventType: "case.created", Case: c, Idempotency: record}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	loaded, ok := reopened.CaseByNumber("PF-1")
	if !ok || loaded.CaseID != "case-1" {
		t.Fatalf("重放未恢复档案: %#v", loaded)
	}
	if saved, ok := reopened.Idempotent("key-1"); !ok || saved.Fingerprint != "fp" {
		t.Fatalf("重放未恢复幂等记录")
	}
	report, err := VerifyLedger(filepath.Join(directory, "events.jsonl"))
	if err != nil || !report.Valid || report.EventCount != 1 {
		t.Fatalf("账本校验失败: %#v %v", report, err)
	}
}

func TestTamperedLedgerIsRejected(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	c := testCase(t)
	if err = store.Commit(CommitRequest{ExpectedVersion: 0, EventType: "case.created", Case: c}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "events.jsonl")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content[len(content)/2] ^= 1
	if err = os.WriteFile(path, content, 0640); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(directory); err == nil {
		t.Fatal("篡改后的账本应被拒绝")
	}
}

func TestTamperedSnapshotIsRejected(t *testing.T) {
	directory := t.TempDir()
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	c := testCase(t)
	if err = store.Commit(CommitRequest{ExpectedVersion: 0, EventType: "case.created", Case: c}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "projection.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content[len(content)/2] ^= 1
	if err = os.WriteFile(path, content, 0640); err != nil {
		t.Fatal(err)
	}
	if _, err = Open(directory); err == nil {
		t.Fatal("篡改后的投影快照应被拒绝")
	}
}
