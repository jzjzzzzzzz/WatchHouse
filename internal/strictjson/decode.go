// Package strictjson decodes bounded JSON while rejecting duplicate and unknown
// object fields. It is for authenticated protocol and policy boundaries, not
// arbitrary schema-less documents.
package strictjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func Decode[T any](input io.Reader, maxBytes int64) (T, error) {
	var result T
	if maxBytes < 1 {
		return result, fmt.Errorf("invalid JSON byte bound")
	}
	body, err := io.ReadAll(io.LimitReader(input, maxBytes+1))
	if err != nil {
		return result, fmt.Errorf("read JSON: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return result, fmt.Errorf("JSON exceeds %d bytes", maxBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := validateValue(decoder); err != nil {
		return result, fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return result, fmt.Errorf("JSON has trailing value")
		}
		return result, fmt.Errorf("invalid JSON: %w", err)
	}
	decoder = json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return result, fmt.Errorf("decode JSON: %w", err)
	}
	return result, nil
}

func validateValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delim {
	case '{':
		keys := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("duplicate object key %q", key)
			}
			keys[key] = struct{}{}
			if err := validateValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("unterminated object")
		}
	case '[':
		for decoder.More() {
			if err := validateValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("unterminated array")
		}
	default:
		return fmt.Errorf("unexpected delimiter")
	}
	return nil
}
