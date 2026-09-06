package schema

import (
	"encoding/json"
	"testing"
)

func decoded(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(JSON()), &m); err != nil {
		t.Fatalf("emitted schema is not valid JSON: %v", err)
	}
	return m
}

func TestSchemaIsValidJSON(t *testing.T) { decoded(t) }

func TestSchemaCoversEveryTopLevelKey(t *testing.T) {
	props, ok := decoded(t)["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema has no properties")
	}
	for _, key := range []string{"app", "watch", "status", "menu"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema does not describe %q", key)
		}
	}
}

func TestSchemaRefusesUnknownTopLevelKeys(t *testing.T) {
	if decoded(t)["additionalProperties"] != false {
		t.Error("schema allows unknown top-level keys; perch itself rejects them")
	}
}
