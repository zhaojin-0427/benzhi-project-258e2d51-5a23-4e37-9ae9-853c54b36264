package application

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"paperfit-release/internal/domain"
	"paperfit-release/internal/eventstore"
)

type Clock func() time.Time

type Service struct {
	store       *eventstore.Store
	now         Clock
	caseCacheMu sync.RWMutex
	caseCache   map[string]json.RawMessage
}

func NewService(store *eventstore.Store) *Service {
	return &Service{store: store, now: func() time.Time { return time.Now().UTC() }, caseCache: map[string]json.RawMessage{}}
}

func NewServiceWithClock(store *eventstore.Store, clock Clock) *Service {
	return &Service{store: store, now: clock, caseCache: map[string]json.RawMessage{}}
}

func randomID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b)
}

func requireRole(ctx Context, allowed ...domain.Role) error {
	if ctx.Actor == "" {
		return domain.NewError("unauthorized", "X-Actor 不能为空")
	}
	for _, role := range allowed {
		if ctx.Role == role {
			return nil
		}
	}
	return domain.NewError("forbidden", "角色 %s 无权执行此操作", ctx.Role)
}

func (s *Service) load(id string) (*domain.SuitabilityCase, error) {
	c, ok := s.store.CaseByID(id)
	if !ok {
		return nil, domain.NewError("not_found", "适配档案不存在")
	}
	return c, nil
}

func fingerprint(operation string, request any) string {
	return domain.HashJSON(struct {
		Operation string `json:"operation"`
		Request   any    `json:"request"`
	}{operation, request})
}

func execute[T any](s *Service, ctx Context, operation string, request any, status int, action func() (T, eventstore.CommitRequest, error)) (T, bool, error) {
	var zero T
	if ctx.IdempotencyKey == "" {
		return zero, false, domain.NewError("idempotency_required", "Idempotency-Key 不能为空")
	}
	fp := fingerprint(operation, request)
	if saved, ok := s.store.Idempotent(ctx.IdempotencyKey); ok {
		if saved.Operation != operation || saved.Fingerprint != fp {
			return zero, false, domain.NewError("idempotency_conflict", "幂等键已用于不同请求")
		}
		var result T
		if err := json.Unmarshal(saved.Response, &result); err != nil {
			return zero, false, err
		}
		return result, true, nil
	}
	result, commit, err := action()
	if err != nil {
		return zero, false, err
	}
	body, err := json.Marshal(result)
	if err != nil {
		return zero, false, err
	}
	commit.Idempotency = &eventstore.IdempotencyRecord{Key: ctx.IdempotencyKey, Operation: operation, Fingerprint: fp, Status: status, Response: body}
	if err := s.store.Commit(commit); err != nil {
		if saved, ok := s.store.Idempotent(ctx.IdempotencyKey); ok && saved.Operation == operation && saved.Fingerprint == fp {
			var stable T
			if decodeErr := json.Unmarshal(saved.Response, &stable); decodeErr == nil {
				return stable, true, nil
			}
		}
		return zero, false, err
	}
	return result, false, nil
}
