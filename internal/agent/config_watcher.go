package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	ErrCodeFileNotFound     = "FILE_NOT_FOUND"
	ErrCodePermissionDenied = "PERMISSION_DENIED"
	ErrCodeReadError        = "READ_ERROR"
)

// ConfigWatcher monitors app.toml and config.toml changes via fsnotify.
type ConfigWatcher struct {
	cfg        *Config
	httpClient *http.Client

	mu       sync.Mutex
	debounce *time.Timer
}

func NewConfigWatcher(cfg *Config) *ConfigWatcher {
	return &ConfigWatcher{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Run watches $NODE_HOME/config and sends updates to {ServiceURL}/config.
func (w *ConfigWatcher) Run(ctx context.Context) {
	if w.cfg.NodeHome == "" || w.cfg.ServiceURL == "" {
		return
	}

	configDir := w.configDir()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config watcher: failed to create watcher: %v\n", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add(configDir); err != nil {
		fmt.Fprintf(os.Stderr, "config watcher: failed to watch %s: %v\n", configDir, err)
		w.sendConfig(ctx)
		return
	}

	w.sendConfig(ctx)

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			filename := filepath.Base(event.Name)
			if filename != "app.toml" && filename != "config.toml" {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			w.debounceSend(ctx, 100*time.Millisecond)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "config watcher: error: %v\n", err)
		}
	}
}

func (w *ConfigWatcher) debounceSend(ctx context.Context, delay time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.debounce != nil {
		w.debounce.Stop()
	}

	w.debounce = time.AfterFunc(delay, func() {
		w.sendConfig(ctx)
	})
}

func (w *ConfigWatcher) configDir() string      { return filepath.Join(w.cfg.NodeHome, "config") }
func (w *ConfigWatcher) appConfigPath() string   { return filepath.Join(w.configDir(), "app.toml") }
func (w *ConfigWatcher) cometConfigPath() string { return filepath.Join(w.configDir(), "config.toml") }
func (w *ConfigWatcher) configURL() string       { return w.cfg.ServiceURL + configEndpoint }

func (w *ConfigWatcher) sendConfig(ctx context.Context) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	appContent, appErr := w.readFile(w.appConfigPath())
	if appErr != nil {
		writer.WriteField("app_error", w.errorToCode(appErr))
	} else if part, err := writer.CreateFormFile("app_config", "app.toml"); err == nil {
		part.Write([]byte(appContent))
	}

	cometContent, cometErr := w.readFile(w.cometConfigPath())
	if cometErr != nil {
		writer.WriteField("comet_error", w.errorToCode(cometErr))
	} else if part, err := writer.CreateFormFile("comet_config", "config.toml"); err == nil {
		part.Write([]byte(cometContent))
	}

	writer.Close()

	if err := w.send(ctx, &buf, writer.FormDataContentType()); err != nil {
		fmt.Fprintf(os.Stderr, "config watcher: send error: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "config watcher: sent configuration update\n")
}

func (w *ConfigWatcher) readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (w *ConfigWatcher) errorToCode(err error) string {
	if os.IsNotExist(err) {
		return ErrCodeFileNotFound
	}
	if os.IsPermission(err) {
		return ErrCodePermissionDenied
	}
	if strings.Contains(err.Error(), "permission denied") {
		return ErrCodePermissionDenied
	}
	return ErrCodeReadError
}

func (w *ConfigWatcher) send(ctx context.Context, body io.Reader, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.configURL(), body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Cosmos-Analyzer-Chain-Id", w.cfg.ChainID)
	req.Header.Set("X-Cosmos-Analyzer-Node-Id", w.cfg.NodeID)
	if w.cfg.AuthKey != "" {
		req.Header.Set("Authorization", "Bearer "+w.cfg.AuthKey)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
