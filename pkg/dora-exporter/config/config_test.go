package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
)

func TestNewConfigFromFileAllocatesConfig(t *testing.T) {
	SetLogger(log.NewNopLogger())

	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("github:\n  token: test-token\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	conf, err := NewConfigFromFile(path)
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	if conf == nil {
		t.Fatal("config is nil")
	}
	if conf.Github.Token != "test-token" {
		t.Fatalf("token = %q, want test-token", conf.Github.Token)
	}
	if conf.Storage.File.Path != defaultExporterFile {
		t.Fatalf("storage path = %q, want %q", conf.Storage.File.Path, defaultExporterFile)
	}
	if conf.Catalog.Mode != "static" {
		t.Fatalf("catalog mode = %q, want static", conf.Catalog.Mode)
	}
}
