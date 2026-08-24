package eventstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func calculateHash(event Event) (string, error) {
	event.Hash = ""
	b, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func cloneJSON[T any](source T) (T, error) {
	var target T
	b, err := json.Marshal(source)
	if err != nil {
		return target, err
	}
	err = json.Unmarshal(b, &target)
	return target, err
}

func snapshotHash(snapshot Snapshot) (string, error) {
	snapshot.ProjectionHash = ""
	b, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
