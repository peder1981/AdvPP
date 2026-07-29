package main

import (
	"testing"
	"encoding/json"
)

func FuzzJSONDecode(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"key":"value"}`))
	f.Add([]byte(`{"nested":{"deep":{"key":123}}}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`[1,2,3,"test"]`))
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var result interface{}
		err := json.Unmarshal(data, &result)
		_ = result
		_ = err
	})
}
