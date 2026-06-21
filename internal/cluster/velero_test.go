package cluster

import (
	"encoding/json"
	"testing"
)

func TestParseBackupsJSON(t *testing.T) {
	rawJSON := `{
		"apiVersion": "v1",
		"kind": "List",
		"items": [
			{
				"metadata": {
					"name": "backup-one"
				},
				"status": {
					"phase": "Completed",
					"startTimestamp": "2026-06-11T12:00:00Z",
					"completionTimestamp": "2026-06-11T12:05:00Z"
				},
				"spec": {
					"ttl": "240h0m0s"
				}
			},
			{
				"metadata": {
					"name": "backup-two"
				},
				"status": {
					"phase": "Failed",
					"startTimestamp": "2026-06-11T13:00:00Z",
					"completionTimestamp": "2026-06-11T13:01:00Z"
				},
				"spec": {
					"ttl": "72h0m0s"
				}
			}
		]
	}`

	var rawList backupsJSONList
	if err := json.Unmarshal([]byte(rawJSON), &rawList); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if len(rawList.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(rawList.Items))
	}

	item1 := rawList.Items[0]
	if item1.Metadata.Name != "backup-one" || item1.Status.Phase != "Completed" || item1.Spec.TTL != "240h0m0s" {
		t.Errorf("Item 1 data mismatch: %+v", item1)
	}

	item2 := rawList.Items[1]
	if item2.Metadata.Name != "backup-two" || item2.Status.Phase != "Failed" || item2.Spec.TTL != "72h0m0s" {
		t.Errorf("Item 2 data mismatch: %+v", item2)
	}
}

func TestGenerateRandomSuffix(t *testing.T) {
	suffix1 := generateRandomSuffix(5)
	suffix2 := generateRandomSuffix(5)

	if len(suffix1) != 5 || len(suffix2) != 5 {
		t.Errorf("Expected length 5, got %d and %d", len(suffix1), len(suffix2))
	}

	if suffix1 == suffix2 {
		t.Errorf("Expected unique suffixes, got duplicates: %s", suffix1)
	}
}
