package eventstore

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"paperfit-release/internal/domain"
)

type Store struct {
	mu           sync.RWMutex
	directory    string
	ledgerPath   string
	snapshotPath string
	sequence     uint64
	lastHash     string
	cases        map[string]*domain.SuitabilityCase
	caseNumbers  map[string]string
	lotCases     map[string][]string
	credentials  map[string]domain.ReleaseCredential
	idempotency  map[string]IdempotencyRecord
}

func Open(directory string) (*Store, error) {
	if directory == "" {
		return nil, errors.New("数据目录不能为空")
	}
	if err := os.MkdirAll(directory, 0750); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	s := &Store{directory: directory, ledgerPath: filepath.Join(directory, "events.jsonl"), snapshotPath: filepath.Join(directory, "projection.json"), cases: map[string]*domain.SuitabilityCase{}, caseNumbers: map[string]string{}, lotCases: map[string][]string{}, credentials: map[string]domain.ReleaseCredential{}, idempotency: map[string]IdempotencyRecord{}}
	restored, err := s.restoreSnapshot()
	if err != nil {
		return nil, err
	}
	if restored {
		report, verifyErr := VerifyLedger(s.ledgerPath)
		if verifyErr != nil {
			return nil, fmt.Errorf("校验事件账本: %w", verifyErr)
		}
		if report.EventCount < s.sequence {
			return nil, errors.New("投影快照领先于事件账本")
		}
		if report.EventCount == s.sequence && report.LastHash != s.lastHash {
			return nil, errors.New("投影快照与事件账本锚点不一致")
		}
		if report.EventCount > s.sequence {
			if err := s.replay(); err != nil {
				return nil, err
			}
			if s.sequence != report.EventCount || s.lastHash != report.LastHash {
				return nil, errors.New("尾部事件重放后锚点与事件账本不一致")
			}
		}
	} else {
		if err := s.replay(); err != nil {
			return nil, err
		}
	}
	if err := s.writeSnapshotLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) restoreSnapshot() (bool, error) {
	f, err := os.Open(s.snapshotPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("打开投影快照: %w", err)
	}
	defer f.Close()
	var snapshot Snapshot
	if err := json.NewDecoder(io.LimitReader(f, 128<<20)).Decode(&snapshot); err != nil {
		return false, fmt.Errorf("解析投影快照: %w", err)
	}
	if snapshot.SchemaVersion != SchemaVersion {
		return false, fmt.Errorf("不支持的快照 schemaVersion: %d", snapshot.SchemaVersion)
	}
	if snapshot.ProjectionHash == "" {
		return false, errors.New("投影快照缺少 projectionHash")
	}
	calculated, err := snapshotHash(snapshot)
	if err != nil {
		return false, fmt.Errorf("计算投影快照哈希: %w", err)
	}
	if calculated != snapshot.ProjectionHash {
		return false, errors.New("投影快照内容哈希不匹配")
	}
	for id, c := range snapshot.Cases {
		if c == nil || c.CaseID != id {
			return false, fmt.Errorf("投影快照档案 %s 的标识无效", id)
		}
		if err := c.ValidateInvariant(); err != nil {
			return false, fmt.Errorf("投影快照档案 %s 不满足聚合不变量: %w", id, err)
		}
	}
	s.sequence = snapshot.LastSequence
	s.lastHash = snapshot.LastHash
	s.cases = snapshot.Cases
	s.credentials = snapshot.Credentials
	s.idempotency = snapshot.Idempotency
	if s.cases == nil {
		s.cases = map[string]*domain.SuitabilityCase{}
	}
	if s.credentials == nil {
		s.credentials = map[string]domain.ReleaseCredential{}
	}
	if s.idempotency == nil {
		s.idempotency = map[string]IdempotencyRecord{}
	}
	s.rebuildIndexes()
	return true, nil
}

func (s *Store) replay() error {
	f, err := os.Open(s.ledgerPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("打开事件账本: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	var line int
	for scanner.Scan() {
		line++
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("事件账本第 %d 行无效: %w", line, err)
		}
		if event.Sequence <= s.sequence {
			continue
		}
		if err := s.validateNextEvent(event); err != nil {
			return fmt.Errorf("事件账本第 %d 行: %w", line, err)
		}
		if err := s.applyEvent(event); err != nil {
			return fmt.Errorf("重放第 %d 行: %w", line, err)
		}
		s.sequence, s.lastHash = event.Sequence, event.Hash
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取事件账本: %w", err)
	}
	return nil
}

func (s *Store) validateNextEvent(event Event) error {
	if event.SchemaVersion != SchemaVersion {
		return fmt.Errorf("不支持的 schemaVersion %d", event.SchemaVersion)
	}
	if event.Sequence != s.sequence+1 {
		return fmt.Errorf("事件序号不连续: %d", event.Sequence)
	}
	if event.PreviousHash != s.lastHash {
		return errors.New("前序哈希不匹配")
	}
	hash, err := calculateHash(event)
	if err != nil {
		return err
	}
	if hash != event.Hash {
		return errors.New("事件校验哈希不匹配")
	}
	return nil
}

func (s *Store) applyEvent(event Event) error {
	var c domain.SuitabilityCase
	if err := json.Unmarshal(event.Payload, &c); err != nil {
		return err
	}
	if err := c.ValidateInvariant(); err != nil {
		return fmt.Errorf("聚合不变量校验失败: %w", err)
	}
	copyCase, err := cloneJSON(&c)
	if err != nil {
		return err
	}
	s.cases[c.CaseID] = copyCase
	if event.Credential != nil {
		s.credentials[event.Credential.CredentialNumber] = *event.Credential
	}
	if event.Idempotency != nil {
		s.idempotency[event.Idempotency.Key] = *event.Idempotency
	}
	s.rebuildIndexes()
	return nil
}

func (s *Store) rebuildIndexes() {
	s.caseNumbers = map[string]string{}
	s.lotCases = map[string][]string{}
	for id, c := range s.cases {
		s.caseNumbers[c.CaseNumber] = id
		s.lotCases[c.PaperLotID] = append(s.lotCases[c.PaperLotID], id)
	}
	for lot := range s.lotCases {
		sort.Strings(s.lotCases[lot])
	}
}

func (s *Store) Commit(request CommitRequest) error {
	if request.Case == nil {
		return errors.New("提交档案不能为空")
	}
	if err := request.Case.ValidateInvariant(); err != nil {
		return fmt.Errorf("拒绝无效聚合: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.Idempotency != nil {
		if existing, exists := s.idempotency[request.Idempotency.Key]; exists {
			if existing.Operation == request.Idempotency.Operation && existing.Fingerprint == request.Idempotency.Fingerprint {
				return domain.NewError("idempotency_replayed", "幂等请求已提交")
			}
			return domain.NewError("idempotency_conflict", "幂等键已用于不同请求")
		}
	}
	current := 0
	if existing := s.cases[request.Case.CaseID]; existing != nil {
		current = existing.Version
	}
	if current != request.ExpectedVersion {
		return domain.NewError("version_conflict", "expectedVersion=%d，当前版本=%d", request.ExpectedVersion, current)
	}
	if existingID, ok := s.caseNumbers[request.Case.CaseNumber]; ok && existingID != request.Case.CaseID {
		return domain.NewError("duplicate_case_number", "caseNumber 已存在")
	}
	if request.Credential != nil {
		if _, exists := s.credentials[request.Credential.CredentialNumber]; exists {
			return domain.NewError("duplicate_credential", "credentialNumber 已存在")
		}
	}
	payload, err := json.Marshal(request.Case)
	if err != nil {
		return err
	}
	event := Event{SchemaVersion: SchemaVersion, Sequence: s.sequence + 1, AggregateID: request.Case.CaseID, AggregateVersion: request.Case.Version, Type: request.EventType, OccurredAt: time.Now().UTC(), PreviousHash: s.lastHash, Payload: payload, Credential: request.Credential, Idempotency: request.Idempotency}
	event.Hash, err = calculateHash(event)
	if err != nil {
		return err
	}
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.ledgerPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("打开事件账本: %w", err)
	}
	if _, err = file.Write(append(line, '\n')); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("追加事件账本: %w", err)
	}
	if closeErr != nil {
		return closeErr
	}
	if err := s.applyEvent(event); err != nil {
		return err
	}
	s.sequence, s.lastHash = event.Sequence, event.Hash
	if err := s.writeSnapshotLocked(); err != nil {
		return fmt.Errorf("更新投影快照: %w", err)
	}
	return nil
}

func (s *Store) writeSnapshotLocked() error {
	snapshot := Snapshot{SchemaVersion: SchemaVersion, LastSequence: s.sequence, LastHash: s.lastHash, Cases: s.cases, Credentials: s.credentials, Idempotency: s.idempotency}
	projectionHash, err := snapshotHash(snapshot)
	if err != nil {
		return err
	}
	snapshot.ProjectionHash = projectionHash
	temporary, err := os.CreateTemp(s.directory, ".projection-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0640); err != nil {
		return err
	}
	if err := os.Rename(name, s.snapshotPath); err != nil {
		return err
	}
	dir, err := os.Open(s.directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
