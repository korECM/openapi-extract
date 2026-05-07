package ordered

import (
	"bytes"
	"encoding/json"
)

type Map struct {
	Keys   []string
	Values map[string]any
}

func New() *Map {
	return &Map{Values: map[string]any{}}
}

func (m *Map) Set(key string, value any) {
	if _, ok := m.Values[key]; !ok {
		m.Keys = append(m.Keys, key)
	}
	m.Values[key] = value
}

func (m *Map) Get(key string) (any, bool) {
	value, ok := m.Values[key]
	return value, ok
}

func (m *Map) Len() int {
	return len(m.Keys)
}

func (m *Map) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, key := range m.Keys {
		if i > 0 {
			b.WriteByte(',')
		}
		keyData, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		valueData, err := json.Marshal(m.Values[key])
		if err != nil {
			return nil, err
		}
		b.Write(keyData)
		b.WriteByte(':')
		b.Write(valueData)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
