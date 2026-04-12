package config

import "testing"

func TestLoadExporterSourcesFromJSON(t *testing.T) {
	sources, err := loadExporterSources("", `[{"source_id":"jp","base_url":"https://jp-xhttp-contabo.svc.plus","expected_node_id":"jp-xhttp-contabo.svc.plus","expected_env":"prod","enabled":true,"timeout_seconds":20}]`)
	if err != nil {
		t.Fatalf("load sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].SourceID != "jp" || sources[0].BaseURL != "https://jp-xhttp-contabo.svc.plus" {
		t.Fatalf("unexpected source %#v", sources[0])
	}
	if sources[0].TimeoutSeconds != 20 {
		t.Fatalf("expected timeout 20, got %d", sources[0].TimeoutSeconds)
	}
}

func TestLoadExporterSourcesFallsBackToLegacyBaseURL(t *testing.T) {
	sources, err := loadExporterSources("http://127.0.0.1:8080", "")
	if err != nil {
		t.Fatalf("load legacy source: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].SourceID != "default" || sources[0].BaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected source %#v", sources[0])
	}
}
