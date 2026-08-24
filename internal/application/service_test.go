package application

import (
	"testing"
	"time"

	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

func TestCreateIdempotencyAndRole(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	service := NewServiceWithClock(store, func() time.Time { return fixed })
	request := CreateCaseRequest{CaseNumber: "PF-TEST", ArtifactRef: "古籍", ArtifactMaterial: "竹纸", RepairArea: "边缘", PaperLot: domain.PaperLot{PaperLotID: "lot", Maker: "纸坊", Origin: "泾县", FiberComposition: "楮皮", ProductionDate: "2026-01-01", NominalWeightGSM: 35, SheetIdentifier: "S1"}}
	ctx := Context{Actor: "修复师", Role: domain.RoleRestorer, IdempotencyKey: "create-1"}
	first, replayed, err := service.CreateCase(ctx, request)
	if err != nil || replayed {
		t.Fatalf("首次创建失败: %v", err)
	}
	second, replayed, err := service.CreateCase(ctx, request)
	if err != nil || !replayed || second.CaseID != first.CaseID {
		t.Fatalf("幂等重放不稳定: %v", err)
	}
	request.RepairArea = "书脊"
	if _, _, err = service.CreateCase(ctx, request); domain.ErrorCode(err) != "idempotency_conflict" {
		t.Fatalf("预期幂等冲突: %v", err)
	}
	ctx = Context{Actor: "检测员", Role: domain.RoleTester, IdempotencyKey: "create-2"}
	if _, _, err = service.CreateCase(ctx, request); domain.ErrorCode(err) != "forbidden" {
		t.Fatalf("预期角色拒绝: %v", err)
	}
}

func TestStaleVersionRejected(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	created, _, err := service.CreateCase(Context{Actor: "修复师", Role: domain.RoleRestorer, IdempotencyKey: "c"}, CreateCaseRequest{CaseNumber: "PF-V", ArtifactRef: "古籍", ArtifactMaterial: "竹纸", RepairArea: "边缘", PaperLot: domain.PaperLot{PaperLotID: "lot", Maker: "纸坊", Origin: "泾县", FiberComposition: "楮皮", ProductionDate: "2026-01-01", NominalWeightGSM: 35, SheetIdentifier: "S1"}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = service.LockPlan(Context{Actor: "检测员", Role: domain.RoleTester, IdempotencyKey: "p"}, created.CaseID, LockPlanRequest{ExpectedVersion: 0, SampleCount: 1})
	if domain.ErrorCode(err) != "version_conflict" {
		t.Fatalf("预期版本冲突: %v", err)
	}
}

func TestReadinessAndAuditQueries(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	created, _, err := service.CreateCase(Context{Actor: "修复师", Role: domain.RoleRestorer, IdempotencyKey: "readiness-create"}, CreateCaseRequest{CaseNumber: "PF-R", ArtifactRef: "古籍", ArtifactMaterial: "竹纸", RepairArea: "边缘", PaperLot: domain.PaperLot{PaperLotID: "lot-r", Maker: "纸坊", Origin: "泾县", FiberComposition: "楮皮", ProductionDate: "2026-01-01", NominalWeightGSM: 35, SheetIdentifier: "S1"}})
	if err != nil {
		t.Fatal(err)
	}
	report, err := service.Readiness(created.CaseID)
	if err != nil || !report.Actions.UpdateDraft || !report.Actions.LockTestPlan {
		t.Fatalf("草稿就绪信息错误: %#v %v", report, err)
	}
	trail, err := service.AuditTrail(created.CaseID, AuditQuery{AfterVersion: 0, Action: "create"})
	if err != nil || trail.Count != 1 || trail.Entries[0].Actor != "修复师" {
		t.Fatalf("审计查询错误: %#v %v", trail, err)
	}
}
