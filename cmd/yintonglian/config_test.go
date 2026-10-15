package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigFileAndOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")
	err := os.WriteFile(path, []byte(`{"root":"site","data_dir":"data","listen":"127.0.0.1:9000","origin":"https://show.zhenxinkeji.top","production":true,"max_upload_mb":200}`), 0600)
	if err != nil {
		t.Fatal(err)
	}
	// Environment variables no longer affect the configuration, even invalid ones.
	t.Setenv("APP_ORIGIN", "https://wrong.example")
	t.Setenv("MAX_UPLOAD_MB", "invalid")
	for _, command := range []string{"serve", "init-admin", "backup"} {
		cfg, err := parseOptions(command, []string{"--config", path})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Root != filepath.Join(dir, "site") || cfg.Data != filepath.Join(dir, "data") || cfg.Origin != "https://show.zhenxinkeji.top" || !cfg.Production || cfg.MaxUploadMB != 200 || cfg.Addr != "127.0.0.1:9000" {
			t.Fatalf("unexpected configuration: %+v", cfg)
		}
	}
	cfg, err := parseOptions("serve", []string{"--config", path, "--production=false", "--origin=", "--addr", "127.0.0.1:9001", "--max-upload-mb", "10"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Production || cfg.Origin != "" || cfg.Addr != "127.0.0.1:9001" || cfg.MaxUploadMB != 10 {
		t.Fatalf("overrides ignored: %+v", cfg)
	}
}

func TestConfigRejectsMistakes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if _, err := parseOptions("serve", []string{"--config", path}); err == nil {
		t.Fatal("missing explicit file accepted")
	}
	for _, content := range []string{`{`, `null`, `[]`, `{} {}`, `{"orgin":"typo"}`, `{"max_upload_mb":0}`, `{"max_upload_mb":4097}`, `{"max_upload_mb":"300"}`, `{"listen":""}`, `{"root":""}`} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := parseOptions("serve", []string{"--config", path}); err == nil {
			t.Fatalf("invalid config accepted: %s", content)
		}
	}
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := parseOptions("backup", []string{"--config", path, "ignored-destination"}); err == nil || !strings.Contains(err.Error(), "unexpected arguments") {
		t.Fatalf("unexpected positional result: %v", err)
	}
	cfg, err := readConfig(filepath.Join(t.TempDir(), "absent.json"), false)
	if err != nil || cfg.Addr != "127.0.0.1:4317" || cfg.MaxUploadMB != 300 {
		t.Fatalf("bad defaults: %+v %v", cfg, err)
	}
}
