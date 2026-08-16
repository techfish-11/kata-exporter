package install


import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)


func TestPatchAndRemovePrometheus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prometheus.yml")
	original := "global:\n  scrape_interval: 15s\nscrape_configs:\n  - job_name: existing\n    static_configs: []\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchPrometheus(path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), managedBegin) != 1 || !strings.Contains(string(b), "job_name: kata-exporter") {
		t.Fatalf("managed scrape job was not inserted correctly:\n%s", b)
	}
	if err := patchPrometheus(path); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if strings.Count(string(b), managedBegin) != 1 {
		t.Fatal("patch is not idempotent")
	}
	if err := removePrometheus(path); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if strings.Contains(string(b), "kata-exporter") || !strings.Contains(string(b), "job_name: existing") {
		t.Fatalf("managed scrape job was not removed cleanly:\n%s", b)
	}
}

