package identity

import (
	"bytes"
	"encoding/json"
	"sort"
)

func CanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	if err := encodeCanonical(generic, buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeCanonical(v any, buf *bytes.Buffer) error {
	switch val := v.(type) {
	case map[string]any:
		buf.WriteByte('{')
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteByte(':')
			if err := encodeCanonical(val[k], buf); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeCanonical(item, buf); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case string, float64, bool, nil:
		b, _ := json.Marshal(val)
		buf.Write(b)
	default:
		// Fallback for numbers, etc.
		b, _ := json.Marshal(val)
		buf.Write(b)
	}
	return nil
}
