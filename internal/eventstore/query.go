package eventstore

import (
	"encoding/json"
	"sort"

	"paperfit-release/internal/domain"
)

func (s *Store) CaseByID(id string) (*domain.SuitabilityCase, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.cases[id]
	if !ok {
		return nil, false
	}
	copyCase, err := cloneJSON(c)
	return copyCase, err == nil
}

func (s *Store) CaseByNumber(number string) (*domain.SuitabilityCase, bool) {
	s.mu.RLock()
	id, ok := s.caseNumbers[number]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return s.CaseByID(id)
}

func (s *Store) CasesByPaperLot(lotID string) []*domain.SuitabilityCase {
	s.mu.RLock()
	ids := append([]string(nil), s.lotCases[lotID]...)
	s.mu.RUnlock()
	result := make([]*domain.SuitabilityCase, 0, len(ids))
	for _, id := range ids {
		if c, ok := s.CaseByID(id); ok {
			result = append(result, c)
		}
	}
	return result
}

func (s *Store) ListCases() []*domain.SuitabilityCase {
	s.mu.RLock()
	ids := make([]string, 0, len(s.cases))
	for id := range s.cases {
		ids = append(ids, id)
	}
	s.mu.RUnlock()
	sort.Strings(ids)
	result := make([]*domain.SuitabilityCase, 0, len(ids))
	for _, id := range ids {
		if c, ok := s.CaseByID(id); ok {
			result = append(result, c)
		}
	}
	return result
}

func (s *Store) Credential(number string) (domain.ReleaseCredential, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.credentials[number]
	if !ok {
		return c, false
	}
	copy, err := cloneJSON(c)
	if err != nil {
		return c, true
	}
	return copy, true
}

func (s *Store) Idempotent(key string) (IdempotencyRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.idempotency[key]
	if ok {
		r.Response = append(json.RawMessage(nil), r.Response...)
	}
	return r, ok
}

func (s *Store) LastSequence() uint64 { s.mu.RLock(); defer s.mu.RUnlock(); return s.sequence }

func (s *Store) Statistics() ProjectionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	states := map[string]int{}
	for _, c := range s.cases {
		states[string(c.State)]++
	}
	return ProjectionStats{SchemaVersion: SchemaVersion, LastSequence: s.sequence, CaseCount: len(s.cases), CredentialCount: len(s.credentials), IdempotencyCount: len(s.idempotency), CasesByState: states}
}
