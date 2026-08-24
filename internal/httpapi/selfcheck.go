package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"paperfit-release/internal/application"
	"paperfit-release/internal/eventstore"
)

type selfcheckEnvelope struct {
	Data json.RawMessage `json:"data"`
}

func RunSelfCheck(addr string, logger *slog.Logger) error {
	directory, err := os.MkdirTemp("", "paperfit-selfcheck-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	store, err := eventstore.Open(directory)
	if err != nil {
		return err
	}
	service := application.NewService(store)
	api := New(service, logger)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("selfcheck 监听失败: %w", err)
	}
	server := NewHTTPServer(addr, api)
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.Serve(listener) }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	baseURL := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: 3 * time.Second}
	request := func(method, path, actor, role, key string, body any) (json.RawMessage, error) {
		var reader io.Reader
		if body != nil {
			encoded, e := json.Marshal(body)
			if e != nil {
				return nil, e
			}
			reader = bytes.NewReader(encoded)
		}
		req, e := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
		if e != nil {
			return nil, e
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if actor != "" {
			req.Header.Set("X-Actor", actor)
			req.Header.Set("X-Role", role)
			req.Header.Set("Idempotency-Key", key)
		}
		resp, e := client.Do(req)
		if e != nil {
			return nil, e
		}
		defer resp.Body.Close()
		content, e := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if e != nil {
			return nil, e
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s %s 返回 %d: %s", method, path, resp.StatusCode, string(content))
		}
		var envelope selfcheckEnvelope
		if e = json.Unmarshal(content, &envelope); e != nil {
			return nil, e
		}
		return envelope.Data, nil
	}
	create := map[string]any{"caseNumber": "PF-SELFCHECK-001", "artifactRef": "古籍-自检", "artifactMaterial": "竹纸", "repairArea": "书页边缘", "paperLot": map[string]any{"paperLotID": "LOT-SELFCHECK", "maker": "自检纸坊", "origin": "安徽泾县", "fiberComposition": "楮皮纤维", "productionDate": "2026-01-01", "nominalWeightGSM": 35, "sheetIdentifier": "SHEET-001"}}
	data, err := request("POST", "/api/v1/suitability-cases", "修复师甲", "restorer", "sc-create", create)
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	var created struct {
		CaseID  string `json:"caseID"`
		Version int    `json:"version"`
	}
	if err = json.Unmarshal(data, &created); err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	path := "/api/v1/suitability-cases/" + created.CaseID
	previewData, err := request("GET", path+"/test-plan/preview?sampleCount=1", "", "", "", nil)
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	var preview struct {
		PreviewDigest string `json:"previewDigest"`
	}
	if err = json.Unmarshal(previewData, &preview); err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	data, err = request("POST", path+"/test-plan", "检测员乙", "tester", "sc-plan", map[string]any{"expectedVersion": created.Version, "sampleCount": 1, "previewDigest": preview.PreviewDigest})
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	var current struct {
		Version      int `json:"version"`
		ReviewRounds []struct {
			ReviewRound      int    `json:"reviewRound"`
			SubmissionDigest string `json:"submissionDigest"`
		} `json:"reviewRounds"`
	}
	_ = json.Unmarshal(data, &current)
	metrics := []struct {
		code, unit string
		value      float64
	}{{"ph", "pH", 7.2}, {"thickness", "mm", 0.10}, {"grammage", "g/m2", 35}, {"color_difference", "DeltaE", 2.1}, {"fiber_direction", "degree", 12}, {"wet_strength", "N/15mm", 1.2}}
	batchMeasurements := make([]any, 0, len(metrics))
	for i, m := range metrics {
		batchMeasurements = append(batchMeasurements, map[string]any{"measurementID": fmt.Sprintf("SC-M-%d", i+1), "metricCode": m.code, "sampleID": "S1", "method": "selfcheck-method", "value": m.value, "unit": m.unit})
	}
	data, err = request("POST", path+"/measurements/batch", "检测员乙", "tester", "sc-measure-batch", map[string]any{"expectedVersion": current.Version, "measurements": batchMeasurements})
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	var batchResult struct {
		CaseVersion int `json:"caseVersion"`
	}
	if err = json.Unmarshal(data, &batchResult); err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	current.Version = batchResult.CaseVersion
	trial := map[string]any{"expectedVersion": current.Version, "assessmentID": "SC-TRIAL", "pasteFormula": "小麦淀粉浆糊 1:5", "wettingDuration": "45秒", "dryingCondition": "室温压平24小时", "reversibilityGrade": "良好", "appearanceChange": "无明显色变", "riskFindings": []any{}, "decision": "usable", "evidenceDigest": "sha256:selfcheck-evidence"}
	data, err = request("POST", path+"/trial-assessment", "修复师甲", "restorer", "sc-trial", trial)
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	_ = json.Unmarshal(data, &current)
	data, err = request("POST", path+"/review-submissions", "修复师甲", "restorer", "sc-submit", map[string]any{"expectedVersion": current.Version, "reason": "自检材料完整"})
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	_ = json.Unmarshal(data, &current)
	round := current.ReviewRounds[len(current.ReviewRounds)-1]
	data, err = request("POST", path+"/reviews", "审核员丙", "reviewer", "sc-return", map[string]any{"expectedVersion": current.Version, "reviewRound": round.ReviewRound, "submissionDigest": round.SubmissionDigest, "approved": false, "reason": "需要补充回退性复测", "issues": []any{map[string]any{"issueID": "SC-ISSUE-1", "item": "回退性", "opinion": "补充湿润回退复测", "blocking": true}}})
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	_ = json.Unmarshal(data, &current)
	revision := map[string]any{"expectedVersion": current.Version, "assessmentID": "SC-TRIAL-2", "supersedesAssessmentID": "SC-TRIAL", "revisionReason": "评审退回后复测", "pasteFormula": "小麦淀粉浆糊 1:5", "wettingDuration": "45秒", "dryingCondition": "室温压平24小时", "reversibilityGrade": "良好", "appearanceChange": "无明显色变", "riskFindings": []any{}, "riskDispositions": []any{}, "decision": "usable", "evidenceDigest": "sha256:selfcheck-retest-evidence"}
	data, err = request("POST", path+"/trial-assessment", "修复师甲", "restorer", "sc-trial-2", revision)
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	_ = json.Unmarshal(data, &current)
	data, err = request("POST", path+"/remediations", "修复师甲", "restorer", "sc-remediate", map[string]any{"expectedVersion": current.Version, "issueID": "SC-ISSUE-1", "measure": "补做湿润回退试验", "evidenceReferences": []any{map[string]any{"type": "trial", "id": "SC-TRIAL-2"}}})
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	_ = json.Unmarshal(data, &current)
	data, err = request("POST", path+"/review-submissions", "修复师甲", "restorer", "sc-resubmit", map[string]any{"expectedVersion": current.Version, "reason": "阻断问题已闭环"})
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	_ = json.Unmarshal(data, &current)
	confirmations := []any{map[string]any{"item": "material_traceability", "confirmed": true}, map[string]any{"item": "measurements", "confirmed": true}, map[string]any{"item": "trial", "confirmed": true}, map[string]any{"item": "risk_closure", "confirmed": true}}
	round = current.ReviewRounds[len(current.ReviewRounds)-1]
	data, err = request("POST", path+"/reviews", "审核员丙", "reviewer", "sc-review", map[string]any{"expectedVersion": current.Version, "reviewRound": round.ReviewRound, "submissionDigest": round.SubmissionDigest, "approved": true, "reason": "复审通过", "confirmations": confirmations, "issues": []any{}})
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	_ = json.Unmarshal(data, &current)
	data, err = request("POST", path+"/freeze", "审核员丙", "reviewer", "sc-freeze", map[string]any{"expectedVersion": current.Version, "reason": "自检冻结"})
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	_ = json.Unmarshal(data, &current)
	precheckData, err := request("GET", path+"/credentials/precheck", "", "", "", nil)
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	var precheck struct {
		RequiredScope map[string]any `json:"requiredScope"`
		Restrictions  []struct {
			RestrictionID string `json:"restrictionID"`
		} `json:"restrictions"`
	}
	if err = json.Unmarshal(precheckData, &precheck); err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	confirmed := make([]string, 0, len(precheck.Restrictions))
	for _, term := range precheck.Restrictions {
		confirmed = append(confirmed, term.RestrictionID)
	}
	data, err = request("POST", path+"/credentials", "审核员丙", "reviewer", "sc-credential", map[string]any{"expectedVersion": current.Version, "usageScope": precheck.RequiredScope, "confirmedRestrictionIDs": confirmed})
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	var issued application.CredentialResult
	if err = json.Unmarshal(data, &issued); err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	verified, err := request("GET", "/api/v1/credentials/"+issued.Credential.CredentialNumber+"/verify", "", "", "", nil)
	if err != nil {
		return shutdownSelfcheck(server, ctx, err)
	}
	var verification application.VerificationResult
	if err = json.Unmarshal(verified, &verification); err != nil || !verification.Valid {
		if err == nil {
			err = fmt.Errorf("凭据验真失败: %s", verification.Status)
		}
		return shutdownSelfcheck(server, ctx, err)
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	select {
	case serveErr := <-serverErr:
		if serveErr != nil && serveErr != http.ErrServerClosed {
			return serveErr
		}
	case <-time.After(time.Second):
		return fmt.Errorf("selfcheck 服务未按时停止")
	}
	return nil
}

func shutdownSelfcheck(server *http.Server, ctx context.Context, cause error) error {
	_ = server.Shutdown(ctx)
	return cause
}
