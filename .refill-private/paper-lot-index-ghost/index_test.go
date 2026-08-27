package paperlotindexghost_test

import (
	"testing"
	"time"

	"paperfit-release/internal/application"
	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

func TestPaperLotIndexMustDropPreviousMembership(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	service := application.NewServiceWithClock(store, func() time.Time { return fixed })
	originalLot := domain.PaperLot{
		PaperLotID: "lot-original", Maker: "甲纸坊", Origin: "泾县",
		FiberComposition: "楮皮", ProductionDate: "2026-01-01",
		NominalWeightGSM: 35, SheetIdentifier: "sheet-original",
	}
	created, _, err := service.CreateCase(
		application.Context{Actor: "修复师", Role: domain.RoleRestorer, IdempotencyKey: "create-index-case"},
		application.CreateCaseRequest{
			CaseNumber: "PF-INDEX-1", ArtifactRef: "古籍-索引",
			ArtifactMaterial: "竹纸", RepairArea: "书页边缘", PaperLot: originalLot,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	replacementLot := domain.PaperLot{
		PaperLotID: "lot-replacement", Maker: "乙纸坊", Origin: "宣城",
		FiberComposition: "檀皮", ProductionDate: "2026-02-01",
		NominalWeightGSM: 37, SheetIdentifier: "sheet-replacement",
	}
	updated, _, err := service.UpdateDraft(
		application.Context{Actor: "修复师", Role: domain.RoleRestorer, IdempotencyKey: "replace-paper-lot"},
		created.CaseID,
		application.UpdateDraftRequest{
			ExpectedVersion: created.Version, ArtifactRef: created.ArtifactRef,
			ArtifactMaterial: created.ArtifactMaterial, RepairArea: created.RepairArea,
			PaperLot: replacementLot, Reason: "更换候选纸批次",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.PaperLotID != replacementLot.PaperLotID {
		t.Fatalf("主投影没有更新纸批次: got %q", updated.PaperLotID)
	}

	oldMembership := service.FindCases("", originalLot.PaperLotID)
	if len(oldMembership) != 0 {
		t.Fatalf("旧批次索引仍返回已迁出的档案: caseID=%s paperLotID=%s", oldMembership[0].CaseID, oldMembership[0].PaperLotID)
	}
	newMembership := service.FindCases("", replacementLot.PaperLotID)
	if len(newMembership) != 1 || newMembership[0].CaseID != created.CaseID {
		t.Fatalf("新批次索引未返回迁入档案: %#v", newMembership)
	}
}
