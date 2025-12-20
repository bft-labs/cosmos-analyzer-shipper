package resourcegating

import (
	"context"
	"runtime"
	"sync"
	"testing"

	"github.com/bft-labs/walship/pkg/walship"
)

// noopLogger implements walship.Logger for testing.
type noopLogger struct{}

func (noopLogger) Debug(msg string, fields ...walship.LogField) {}
func (noopLogger) Info(msg string, fields ...walship.LogField)  {}
func (noopLogger) Warn(msg string, fields ...walship.LogField)  {}
func (noopLogger) Error(msg string, fields ...walship.LogField) {}

func TestPlugin_Name(t *testing.T) {
	p := New(DefaultConfig())
	if p.Name() != "resourcegating" {
		t.Errorf("Name() = %v, want resourcegating", p.Name())
	}
}

func TestPlugin_Initialize(t *testing.T) {
	p := New(DefaultConfig())
	logger := &noopLogger{}

	ctx := context.Background()
	err := p.Initialize(ctx, walship.PluginConfig{
		Logger: logger,
	})

	if err != nil {
		t.Errorf("Initialize() = %v, want nil", err)
	}
}

func TestPlugin_Shutdown(t *testing.T) {
	p := New(DefaultConfig())

	ctx := context.Background()
	if err := p.Initialize(ctx, walship.PluginConfig{Logger: &noopLogger{}}); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() = %v, want nil", err)
	}
}

func TestPlugin_ResourcesOK(t *testing.T) {
	p := New(DefaultConfig())

	ctx := context.Background()
	if err := p.Initialize(ctx, walship.PluginConfig{Logger: &noopLogger{}}); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// ResourcesOK should always return true in the current simple implementation
	if !p.ResourcesOK() {
		t.Error("ResourcesOK() should return true")
	}
}

func TestPlugin_ResourcesOK_Concurrent(t *testing.T) {
	p := New(DefaultConfig())

	ctx := context.Background()
	if err := p.Initialize(ctx, walship.PluginConfig{Logger: &noopLogger{}}); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Concurrent calls to ResourcesOK should be safe
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.ResourcesOK()
		}()
	}
	wg.Wait()
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.CPUThreshold != 0.85 {
		t.Errorf("Default CPUThreshold = %v, want 0.85", cfg.CPUThreshold)
	}
	if cfg.NetThreshold != 0.70 {
		t.Errorf("Default NetThreshold = %v, want 0.70", cfg.NetThreshold)
	}
	if cfg.IfaceSpeedMbps != 1000 {
		t.Errorf("Default IfaceSpeedMbps = %v, want 1000", cfg.IfaceSpeedMbps)
	}
	if cfg.Iface != "" {
		t.Errorf("Default Iface = %v, want empty", cfg.Iface)
	}
}

func TestNew_DefaultsZeroValues(t *testing.T) {
	// Zero values should be replaced with defaults
	p := New(Config{})

	if p.cpuThreshold != 0.85 {
		t.Errorf("cpuThreshold = %v, want 0.85", p.cpuThreshold)
	}
	if p.netThreshold != 0.70 {
		t.Errorf("netThreshold = %v, want 0.70", p.netThreshold)
	}
	if p.ifaceSpeed != 1000 {
		t.Errorf("ifaceSpeed = %v, want 1000", p.ifaceSpeed)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	cfg := Config{
		CPUThreshold:   0.95,
		NetThreshold:   0.80,
		Iface:          "eth0",
		IfaceSpeedMbps: 10000,
	}
	p := New(cfg)

	if p.cpuThreshold != 0.95 {
		t.Errorf("cpuThreshold = %v, want 0.95", p.cpuThreshold)
	}
	if p.netThreshold != 0.80 {
		t.Errorf("netThreshold = %v, want 0.80", p.netThreshold)
	}
	if p.iface != "eth0" {
		t.Errorf("iface = %v, want eth0", p.iface)
	}
	if p.ifaceSpeed != 10000 {
		t.Errorf("ifaceSpeed = %v, want 10000", p.ifaceSpeed)
	}
}

func TestWithResourceGating(t *testing.T) {
	// Test that WithResourceGating returns a valid Option
	opt := WithResourceGating(DefaultConfig())
	if opt == nil {
		t.Error("WithResourceGating() returned nil")
	}

	// The option should be a function that can be applied
	// (we can't easily test this without access to walship internals)
}

func TestPlugin_GoroutineHeuristic(t *testing.T) {
	p := New(DefaultConfig())

	ctx := context.Background()
	if err := p.Initialize(ctx, walship.PluginConfig{Logger: &noopLogger{}}); err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	// Get baseline goroutine count
	baselineGoroutines := runtime.NumGoroutine()
	numCPU := runtime.NumCPU()

	// ResourcesOK uses goroutine count as a heuristic
	// It should still return true even if goroutines > 10x CPU
	// (the current implementation is just informational)
	result := p.ResourcesOK()
	if !result {
		t.Errorf("ResourcesOK() = false, want true (baseline goroutines: %d, CPUs: %d)",
			baselineGoroutines, numCPU)
	}
}

func TestPlugin_NilLoggerInResourcesOK(t *testing.T) {
	p := New(DefaultConfig())
	// Don't initialize, so logger is nil

	// Should not panic even with nil logger
	result := p.ResourcesOK()
	if !result {
		t.Error("ResourcesOK() should return true even without initialization")
	}
}

func TestPlugin_MultipleInitializeShutdown(t *testing.T) {
	p := New(DefaultConfig())
	ctx := context.Background()
	cfg := walship.PluginConfig{Logger: &noopLogger{}}

	// Multiple initialize calls should not cause issues
	for i := 0; i < 3; i++ {
		if err := p.Initialize(ctx, cfg); err != nil {
			t.Errorf("Initialize() iteration %d failed: %v", i, err)
		}
	}

	// Multiple shutdown calls should not cause issues
	for i := 0; i < 3; i++ {
		if err := p.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() iteration %d failed: %v", i, err)
		}
	}
}

// Ensure Plugin implements walship.Plugin
func TestPlugin_ImplementsInterface(t *testing.T) {
	var _ walship.Plugin = (*Plugin)(nil)
}
