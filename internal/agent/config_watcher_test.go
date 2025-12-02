package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConfigWatcher_SendConfig(t *testing.T) {
	// Create temp config directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create app.toml
	appToml := `[api]
enable = true
address = "tcp://0.0.0.0:1317"
`
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(appToml), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}

	// Create config.toml
	configToml := `[p2p]
laddr = "tcp://0.0.0.0:26656"
seeds = ""
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configToml), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	// Track received multipart data
	var receivedAppConfig string
	var receivedCometConfig string
	var receivedAppError string
	var receivedCometError string
	var receivedHeaders http.Header

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ingest/config" {
			t.Errorf("Path = %v, want /v1/ingest/config", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Method = %v, want POST", r.Method)
		}

		receivedHeaders = r.Header.Clone()

		// Verify Content-Type is multipart/form-data
		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			t.Errorf("Content-Type = %v, want multipart/form-data", contentType)
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
		}

		// Get file: app_config
		if file, _, err := r.FormFile("app_config"); err == nil {
			data, _ := io.ReadAll(file)
			receivedAppConfig = string(data)
			file.Close()
		}

		// Get file: comet_config
		if file, _, err := r.FormFile("comet_config"); err == nil {
			data, _ := io.ReadAll(file)
			receivedCometConfig = string(data)
			file.Close()
		}

		// Get fields: app_error, comet_error
		receivedAppError = r.FormValue("app_error")
		receivedCometError = r.FormValue("comet_error")

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
		AuthKey:    "secret",
	}

	watcher := NewConfigWatcher(cfg)

	// Send config
	watcher.sendConfig(context.Background())

	// Verify headers
	if receivedHeaders.Get("X-Cosmos-Analyzer-Chain-Id") != "test-chain" {
		t.Errorf("Chain-Id header = %v, want test-chain", receivedHeaders.Get("X-Cosmos-Analyzer-Chain-Id"))
	}
	if receivedHeaders.Get("X-Cosmos-Analyzer-Node-Id") != "test-node" {
		t.Errorf("Node-Id header = %v, want test-node", receivedHeaders.Get("X-Cosmos-Analyzer-Node-Id"))
	}
	if receivedHeaders.Get("Authorization") != "Bearer secret" {
		t.Errorf("Authorization header = %v, want Bearer secret", receivedHeaders.Get("Authorization"))
	}

	// Verify app config was received as file
	if receivedAppConfig == "" {
		t.Error("AppConfig should not be empty")
	}
	if receivedAppError != "" {
		t.Errorf("AppError should be empty, got %v", receivedAppError)
	}

	// Verify comet config was received as file
	if receivedCometConfig == "" {
		t.Error("CometConfig should not be empty")
	}
	if receivedCometError != "" {
		t.Errorf("CometError should be empty, got %v", receivedCometError)
	}
}

func TestConfigWatcher_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	// Don't create any config files

	var receivedAppConfig string
	var receivedCometConfig string
	var receivedAppError string
	var receivedCometError string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
		}

		// Get files (should not exist)
		if file, _, err := r.FormFile("app_config"); err == nil {
			data, _ := io.ReadAll(file)
			receivedAppConfig = string(data)
			file.Close()
		}
		if file, _, err := r.FormFile("comet_config"); err == nil {
			data, _ := io.ReadAll(file)
			receivedCometConfig = string(data)
			file.Close()
		}

		// Get error fields
		receivedAppError = r.FormValue("app_error")
		receivedCometError = r.FormValue("comet_error")

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)
	watcher.sendConfig(context.Background())

	// Should have error codes for missing files
	if receivedAppError != ErrCodeFileNotFound {
		t.Errorf("AppError = %v, want %v", receivedAppError, ErrCodeFileNotFound)
	}
	if receivedCometError != ErrCodeFileNotFound {
		t.Errorf("CometError = %v, want %v", receivedCometError, ErrCodeFileNotFound)
	}
	if receivedAppConfig != "" {
		t.Errorf("AppConfig should be empty when file is missing")
	}
	if receivedCometConfig != "" {
		t.Errorf("CometConfig should be empty when file is missing")
	}
}

func TestConfigWatcher_FsnotifyDetectsChanges(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	appTomlPath := filepath.Join(configDir, "app.toml")
	if err := os.WriteFile(appTomlPath, []byte(`enable = true`), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`laddr = "tcp://0.0.0.0:26656"`), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var mu sync.Mutex
	sendCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sendCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL,
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watcher in background
	go watcher.Run(ctx)

	// Wait for initial send
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	initialCount := sendCount
	mu.Unlock()

	if initialCount < 1 {
		t.Errorf("sendCount = %d, want >= 1 (initial send)", initialCount)
	}

	// Modify app.toml
	if err := os.WriteFile(appTomlPath, []byte(`enable = false`), 0644); err != nil {
		t.Fatalf("Failed to modify app.toml: %v", err)
	}

	// Wait for fsnotify to detect change and debounce to fire
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	afterChangeCount := sendCount
	mu.Unlock()

	if afterChangeCount <= initialCount {
		t.Errorf("sendCount after change = %d, want > %d", afterChangeCount, initialCount)
	}
}

func TestConfigWatcher_URLConstruction(t *testing.T) {
	// Test that base URL is correctly constructed to full path for config endpoint
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create app.toml
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create app.toml: %v", err)
	}

	// Create config.toml
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`test = true`), 0644); err != nil {
		t.Fatalf("Failed to create config.toml: %v", err)
	}

	var requestPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &Config{
		NodeHome:   tmpDir,
		ServiceURL: ts.URL, // Base URL only, no /v1/ingest/config
		ChainID:    "test-chain",
		NodeID:     "test-node",
	}

	watcher := NewConfigWatcher(cfg)
	watcher.sendConfig(context.Background())

	expectedPath := "/v1/ingest/config"
	if requestPath != expectedPath {
		t.Errorf("Request path = %v, want %v", requestPath, expectedPath)
	}
}

