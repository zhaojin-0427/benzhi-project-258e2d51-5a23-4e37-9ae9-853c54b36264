package stale_case_detail_cache_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"paperfit-release/internal/application"
	"paperfit-release/internal/eventstore"
	"paperfit-release/internal/httpapi"
)

type responseEnvelope struct {
	Data json.RawMessage `json:"data"`
}

type caseData struct {
	CaseID      string `json:"caseID"`
	ArtifactRef string `json:"artifactRef"`
	Version     int    `json:"version"`
}

func TestCaseDetailCacheMustFollowCommittedVersion(t *testing.T) {
	store, err := eventstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(httpapi.New(application.NewService(store), logger))
	defer server.Close()

	created := writeCase(t, server.URL+"/api/v1/suitability-cases", http.MethodPost, "cache-create", map[string]any{
		"caseNumber": "PF-CACHE-001", "artifactRef": "古籍原记录", "artifactMaterial": "竹纸", "repairArea": "书页边缘",
		"paperLot": map[string]any{"paperLotID": "LOT-CACHE", "maker": "测试纸坊", "origin": "安徽泾县", "fiberComposition": "楮皮纤维", "productionDate": "2026-01-01", "nominalWeightGSM": 35, "sheetIdentifier": "SHEET-CACHE"},
	})
	caseURL := server.URL + "/api/v1/suitability-cases/" + created.CaseID
	first := readCase(t, caseURL)
	if first.Version != created.Version {
		t.Fatalf("首次详情版本错误: got %d want %d", first.Version, created.Version)
	}

	updated := writeCase(t, caseURL+"/draft", http.MethodPut, "cache-update", map[string]any{
		"expectedVersion": first.Version, "artifactRef": "古籍修订记录", "artifactMaterial": "竹纸", "repairArea": "书页边缘", "reason": "补充著录",
		"paperLot": map[string]any{"paperLotID": "LOT-CACHE", "maker": "测试纸坊", "origin": "安徽泾县", "fiberComposition": "楮皮纤维", "productionDate": "2026-01-01", "nominalWeightGSM": 35, "sheetIdentifier": "SHEET-CACHE"},
	})
	latest := readCase(t, caseURL)
	if latest.Version != updated.Version || latest.ArtifactRef != "古籍修订记录" {
		t.Fatalf("成功写入版本 %d 后详情缓存仍返回 version=%d artifactRef=%q", updated.Version, latest.Version, latest.ArtifactRef)
	}
}

func writeCase(t *testing.T, url, method, key string, body any) caseData {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(method, url, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Actor", "修复师甲")
	request.Header.Set("X-Role", "restorer")
	request.Header.Set("Idempotency-Key", key)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("写请求返回 %d: %s", response.StatusCode, payload)
	}
	return decodeCase(t, response)
}

func readCase(t *testing.T, url string) caseData {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("详情请求返回 %d", response.StatusCode)
	}
	var view struct {
		Case caseData `json:"case"`
	}
	decodeEnvelope(t, response, &view)
	return view.Case
}

func decodeCase(t *testing.T, response *http.Response) caseData {
	t.Helper()
	var result caseData
	decodeEnvelope(t, response, &result)
	return result
}

func decodeEnvelope(t *testing.T, response *http.Response, target any) {
	t.Helper()
	var envelope responseEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatal(err)
	}
}
