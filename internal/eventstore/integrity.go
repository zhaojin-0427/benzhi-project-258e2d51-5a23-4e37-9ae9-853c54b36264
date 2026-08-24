package eventstore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type IntegrityReport struct {
	Valid      bool   `json:"valid"`
	EventCount uint64 `json:"eventCount"`
	LastHash   string `json:"lastHash"`
}

func VerifyLedger(path string) (IntegrityReport, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return IntegrityReport{Valid: true}, nil
	}
	if err != nil {
		return IntegrityReport{}, err
	}
	defer f.Close()
	var sequence uint64
	var previous string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return IntegrityReport{}, err
		}
		if event.Sequence != sequence+1 || event.PreviousHash != previous {
			return IntegrityReport{}, fmt.Errorf("事件链在序号 %d 断裂", event.Sequence)
		}
		hash, err := calculateHash(event)
		if err != nil || hash != event.Hash {
			return IntegrityReport{}, fmt.Errorf("事件 %d 哈希无效", event.Sequence)
		}
		sequence, previous = event.Sequence, event.Hash
	}
	if err := scanner.Err(); err != nil {
		return IntegrityReport{}, err
	}
	return IntegrityReport{Valid: true, EventCount: sequence, LastHash: previous}, nil
}
