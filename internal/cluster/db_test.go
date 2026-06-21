package cluster

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseTargetTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "Valid RFC3339 UTC",
			input:   "2026-06-12T18:00:00Z",
			want:    time.Date(2026, 6, 12, 18, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "Valid RFC3339 Offset",
			input:   "2026-06-12T18:00:00+02:00",
			want:    time.Date(2026, 6, 12, 16, 0, 0, 0, time.UTC), // 18:00+0200 is 16:00 UTC
			wantErr: false,
		},
		{
			name:    "Valid Simple UTC",
			input:   "2026-06-12 18:00:00",
			want:    time.Date(2026, 6, 12, 18, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "Invalid Date",
			input:   "invalid-date-format",
			wantErr: true,
		},
		{
			name:    "Partial Date Only",
			input:   "2026-06-12",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTargetTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTargetTime() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if !got.Equal(tt.want) {
					t.Errorf("ParseTargetTime() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestGeneratePITRManifest(t *testing.T) {
	sourceClusterRaw := map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name":      "clandestino-db",
			"namespace": "clandestino-dev",
		},
		"spec": map[string]interface{}{
			"instances": 3,
			"imageName": "ghcr.io/cloudnative-pg/postgresql:18",
			"storage": map[string]interface{}{
				"size": "10Gi",
			},
			"backup": map[string]interface{}{
				"barmanObjectStore": map[string]interface{}{
					"destinationPath": "s3://homelab-clandestino/barman",
					"s3Credentials": map[string]interface{}{
						"accessKeyId": map[string]interface{}{
							"name": "aws-creds",
							"key":  "access-key",
						},
					},
				},
			},
			"postgresql": map[string]interface{}{
				"parameters": map[string]interface{}{
					"password_encryption": "scram-sha-256",
				},
			},
		},
	}

	targetName := "clandestino-db-pitr"
	targetTime := time.Date(2026, 6, 12, 18, 0, 0, 0, time.UTC)

	manifestJSON, err := GeneratePITRManifest(sourceClusterRaw, targetName, targetTime)
	if err != nil {
		t.Fatalf("GeneratePITRManifest failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(manifestJSON), &parsed); err != nil {
		t.Fatalf("Failed to parse generated manifest: %v", err)
	}

	// Verify top-level metadata
	metadata, ok := parsed["metadata"].(map[string]interface{})
	if !ok || metadata["name"] != targetName {
		t.Errorf("Expected metadata name to be %q, got %+v", targetName, metadata)
	}

	spec, ok := parsed["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("Spec block missing in recovery manifest")
	}

	// Verify deep-copied values
	if spec["instances"].(float64) != 3 {
		t.Errorf("Expected instances to be 3, got %v", spec["instances"])
	}
	if spec["imageName"].(string) != "ghcr.io/cloudnative-pg/postgresql:18" {
		t.Errorf("Expected imageName to be correct, got %v", spec["imageName"])
	}

	postgresql, ok := spec["postgresql"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected postgresql section to be deep-copied")
	} else {
		params, _ := postgresql["parameters"].(map[string]interface{})
		if params["password_encryption"] != "scram-sha-256" {
			t.Errorf("Expected postgresql parameters to be preserved, got %+v", params)
		}
	}

	// Verify recovery bootstrap section
	bootstrap, ok := spec["bootstrap"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected bootstrap section in spec")
	}
	recovery, ok := bootstrap["recovery"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected recovery section in bootstrap")
	}
	if recovery["source"] != targetName+"-recovery-source" {
		t.Errorf("Expected recovery source to be target-prefixed, got %v", recovery["source"])
	}

	recoveryTarget, ok := recovery["recoveryTarget"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected recoveryTarget section")
	}
	if recoveryTarget["targetTime"] != targetTime.Format(time.RFC3339) {
		t.Errorf("Expected recoveryTarget time to be %q, got %v", targetTime.Format(time.RFC3339), recoveryTarget["targetTime"])
	}

	// Verify externalClusters section
	externalClusters, ok := spec["externalClusters"].([]interface{})
	if !ok || len(externalClusters) != 1 {
		t.Fatalf("Expected exactly 1 external cluster, got %+v", spec["externalClusters"])
	}
	ext := externalClusters[0].(map[string]interface{})
	if ext["name"] != targetName+"-recovery-source" {
		t.Errorf("Expected external cluster name to match recovery source")
	}
	barmanStore, ok := ext["barmanObjectStore"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected barmanObjectStore to be present in external cluster")
	} else {
		if barmanStore["destinationPath"] != "s3://homelab-clandestino/barman" {
			t.Errorf("Expected barmanObjectStore destination path to be correct, got %v", barmanStore["destinationPath"])
		}
		if barmanStore["serverName"] != "clandestino-db" {
			t.Errorf("Expected barmanObjectStore serverName to be 'clandestino-db', got %v", barmanStore["serverName"])
		}
	}
}

func TestGeneratePITRManifestNoBarman(t *testing.T) {
	invalidClusterRaw := map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name": "clandestino-db",
		},
		"spec": map[string]interface{}{
			"instances": 3,
		},
	}

	_, err := GeneratePITRManifest(invalidClusterRaw, "fail-target", time.Now())
	if err == nil || !strings.Contains(err.Error(), "barman cloud plugin") {
		t.Errorf("Expected error indicating lack of barman cloud plugin, got: %v", err)
	}
}

func TestGeneratePITRManifestPlugin(t *testing.T) {
	sourceClusterRaw := map[string]interface{}{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name":      "clandestino-db",
			"namespace": "clandestino-dev",
		},
		"spec": map[string]interface{}{
			"instances": 3,
			"imageName": "ghcr.io/cloudnative-pg/postgresql:18",
			"plugins": []interface{}{
				map[string]interface{}{
					"name": "barman-cloud.cloudnative-pg.io",
					"parameters": map[string]interface{}{
						"barmanObjectName": "clandestino-db-backup-store",
					},
				},
			},
		},
	}

	targetName := "clandestino-db-pitr"
	targetTime := time.Date(2026, 6, 12, 18, 0, 0, 0, time.UTC)

	manifestJSON, err := GeneratePITRManifest(sourceClusterRaw, targetName, targetTime)
	if err != nil {
		t.Fatalf("GeneratePITRManifest failed with plugin: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(manifestJSON), &parsed); err != nil {
		t.Fatalf("Failed to parse generated manifest: %v", err)
	}

	spec, ok := parsed["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("Spec block missing in recovery manifest")
	}

	// Verify externalClusters section contains plugin
	externalClusters, ok := spec["externalClusters"].([]interface{})
	if !ok || len(externalClusters) != 1 {
		t.Fatalf("Expected exactly 1 external cluster, got %+v", spec["externalClusters"])
	}
	ext := externalClusters[0].(map[string]interface{})
	if ext["name"] != targetName+"-recovery-source" {
		t.Errorf("Expected external cluster name to match recovery source")
	}
	plugin, ok := ext["plugin"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected plugin object, got %+v", ext["plugin"])
	}
	if plugin["name"] != "barman-cloud.cloudnative-pg.io" {
		t.Errorf("Expected plugin name to be barman-cloud, got %v", plugin["name"])
	}
	params, ok := plugin["parameters"].(map[string]interface{})
	if !ok || params["barmanObjectName"] != "clandestino-db-backup-store" {
		t.Errorf("Expected barmanObjectName to be 'clandestino-db-backup-store', got %+v", params)
	}
	if params["serverName"] != "clandestino-db" {
		t.Errorf("Expected params serverName to be 'clandestino-db', got %v", params["serverName"])
	}
}
