package eventstore

import (
	"encoding/json"
	"time"

	"paperfit-release/internal/domain"
)

const SchemaVersion = 1

type Event struct {
	SchemaVersion    int                       `json:"schemaVersion"`
	Sequence         uint64                    `json:"sequence"`
	AggregateID      string                    `json:"aggregateID"`
	AggregateVersion int                       `json:"aggregateVersion"`
	Type             string                    `json:"type"`
	OccurredAt       time.Time                 `json:"occurredAt"`
	PreviousHash     string                    `json:"previousHash"`
	Payload          json.RawMessage           `json:"payload"`
	Credential       *domain.ReleaseCredential `json:"credential,omitempty"`
	Idempotency      *IdempotencyRecord        `json:"idempotency,omitempty"`
	Hash             string                    `json:"hash"`
}

type IdempotencyRecord struct {
	Key         string          `json:"key"`
	Operation   string          `json:"operation"`
	Fingerprint string          `json:"fingerprint"`
	Status      int             `json:"status"`
	Response    json.RawMessage `json:"response"`
}

type Snapshot struct {
	SchemaVersion  int                                 `json:"schemaVersion"`
	LastSequence   uint64                              `json:"lastSequence"`
	LastHash       string                              `json:"lastHash"`
	ProjectionHash string                              `json:"projectionHash"`
	Cases          map[string]*domain.SuitabilityCase  `json:"cases"`
	Credentials    map[string]domain.ReleaseCredential `json:"credentials"`
	Idempotency    map[string]IdempotencyRecord        `json:"idempotency"`
}

type ProjectionStats struct {
	SchemaVersion    int            `json:"schemaVersion"`
	LastSequence     uint64         `json:"lastSequence"`
	CaseCount        int            `json:"caseCount"`
	CredentialCount  int            `json:"credentialCount"`
	IdempotencyCount int            `json:"idempotencyCount"`
	CasesByState     map[string]int `json:"casesByState"`
}

type CommitRequest struct {
	ExpectedVersion int
	EventType       string
	Case            *domain.SuitabilityCase
	Credential      *domain.ReleaseCredential
	Idempotency     *IdempotencyRecord
}
