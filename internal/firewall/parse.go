package firewall

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
)

type document struct {
	Nftables []json.RawMessage `json:"nftables"`
}

type namedObject struct {
	Family string `json:"family"`
	Table  string `json:"table"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Hook   string `json:"hook"`
	Prio   any    `json:"prio"`
	Policy string `json:"policy"`
}

func Parse(raw []byte, observedAt time.Time) (Snapshot, error) {
	if len(raw) == 0 || len(raw) > MaxRulesetBytes {
		return Snapshot{}, fmt.Errorf("nftables output size must be 1..%d bytes", MaxRulesetBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc document
	if err := dec.Decode(&doc); err != nil {
		return Snapshot{}, fmt.Errorf("decode nftables document: %w", err)
	}
	if len(doc.Nftables) > MaxEntries {
		return Snapshot{}, fmt.Errorf("nftables entry count exceeds %d", MaxEntries)
	}
	sum := sha256.Sum256(raw)
	s := Snapshot{SchemaVersion: 1, ObservedAt: observedAt.UTC(), SHA256: hex.EncodeToString(sum[:]), Tables: []Table{}, BaseChains: []BaseChain{}}
	for _, entry := range doc.Nftables {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(entry, &fields); err != nil || len(fields) != 1 {
			return Snapshot{}, fmt.Errorf("each nftables entry must contain exactly one object")
		}
		for kind, body := range fields {
			if err := s.add(kind, body); err != nil {
				return Snapshot{}, err
			}
		}
	}
	sort.Slice(s.Tables, func(i, j int) bool {
		return s.Tables[i].Family+"\x00"+s.Tables[i].Name < s.Tables[j].Family+"\x00"+s.Tables[j].Name
	})
	sort.Slice(s.BaseChains, func(i, j int) bool {
		a, b := s.BaseChains[i], s.BaseChains[j]
		return a.Family+"\x00"+a.Table+"\x00"+a.Name < b.Family+"\x00"+b.Table+"\x00"+b.Name
	})
	return s, nil
}

func (s *Snapshot) add(kind string, body json.RawMessage) error {
	switch kind {
	case "metainfo":
		return nil
	case "rule":
		s.RuleCount++
		return nil
	case "set":
		s.SetCount++
		return nil
	case "map":
		s.MapCount++
		return nil
	case "table", "chain":
		var object namedObject
		if err := json.Unmarshal(body, &object); err != nil {
			return fmt.Errorf("decode %s: %w", kind, err)
		}
		if err := validateName(object.Family, "family"); err != nil {
			return err
		}
		if err := validateName(object.Name, "name"); err != nil {
			return err
		}
		if kind == "table" {
			s.Tables = append(s.Tables, Table{Family: object.Family, Name: object.Name})
			return nil
		}
		if object.Hook == "" { // regular chain, not a base-chain boundary
			return nil
		}
		if err := validateName(object.Table, "table"); err != nil {
			return err
		}
		priority, err := scalar(object.Prio)
		if err != nil {
			return fmt.Errorf("chain priority: %w", err)
		}
		s.BaseChains = append(s.BaseChains, BaseChain{object.Family, object.Table, object.Name, object.Type, object.Hook, priority, object.Policy})
		return nil
	default:
		s.OtherCount++
		return nil
	}
}

func scalar(v any) (string, error) {
	switch value := v.(type) {
	case float64:
		if value != float64(int64(value)) {
			return "", fmt.Errorf("must be an integer")
		}
		return strconv.FormatInt(int64(value), 10), nil
	case string:
		if len(value) > 64 {
			return "", fmt.Errorf("string is too long")
		}
		return value, nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported scalar")
	}
}

func validateName(value, field string) error {
	if value == "" || len(value) > 128 {
		return fmt.Errorf("%s must be 1..128 bytes", field)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%s contains a control character", field)
		}
	}
	return nil
}
