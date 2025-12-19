# Migration Guide: Monolithic to Modular Architecture

This guide helps you migrate from the monolithic walship package to the new modular architecture.

## Overview

Walship has been reorganized into independent modules under `pkg/`:

| Module | Purpose |
|--------|---------|
| `pkg/walship` | Main facade - backward compatible |
| `pkg/wal` | WAL reading |
| `pkg/batch` | Frame batching |
| `pkg/sender` | HTTP transmission |
| `pkg/state` | State persistence |
| `pkg/log` | Logging abstraction |
| `pkg/lifecycle` | Orchestration |

Optional plugins under `plugins/`:

| Plugin | Purpose |
|--------|---------|
| `plugins/resourcegating` | CPU/goroutine monitoring |
| `plugins/walcleanup` | WAL file cleanup |
| `plugins/configwatcher` | Config file monitoring |

## No Changes Required (Backward Compatibility)

If you're using the existing `pkg/walship` API, **no changes are required**. The facade maintains 100% backward compatibility:

```go
// This still works exactly the same
import "github.com/bft-labs/walship/pkg/walship"

cfg := walship.Config{
    WALDir:  "/path/to/wal",
    AuthKey: "your-key",
}
agent, err := walship.New(cfg)
```

## Optional: Selective Module Import

If you only need specific functionality, you can now import modules independently:

### Before (Monolithic)

```go
import "github.com/bft-labs/walship/pkg/walship"

// Importing walship pulled in everything: HTTP, batching, lifecycle, etc.
```

### After (Selective)

```go
// Import only what you need
import (
    "github.com/bft-labs/walship/pkg/wal"
    "github.com/bft-labs/walship/pkg/batch"
)

// Use WAL reader without HTTP dependencies
reader := wal.NewIndexReader("/path/to/wal", logger)

// Use batching without sender
batcher := batch.NewBatcher(4<<20, 5*time.Second, 10*time.Second)
```

## Optional: Adding Plugins

Previously, features like resource gating were always enabled. Now they're opt-in:

### Before (Always Enabled)

```go
// Resource gating was built into the agent
agent, err := walship.New(cfg)
```

### After (Opt-In)

```go
import (
    "github.com/bft-labs/walship/pkg/walship"
    "github.com/bft-labs/walship/plugins/resourcegating"
)

// Resource gating is now an explicit plugin
agent, err := walship.New(cfg,
    resourcegating.WithResourceGating(resourcegating.DefaultConfig()),
)
```

## Custom Implementations

The modular architecture exposes interfaces for custom implementations:

### Custom Sender

```go
import "github.com/bft-labs/walship/pkg/sender"

// Implement sender.Sender interface
type MyS3Sender struct { /* ... */ }

func (s *MyS3Sender) Send(ctx context.Context, b *batch.Batch, m sender.Metadata) error {
    // Upload to S3 instead of HTTP
}
```

### Custom Logger

```go
import "github.com/bft-labs/walship/pkg/log"

// Implement log.Logger interface
type MyZapLogger struct { /* ... */ }

func (l *MyZapLogger) Debug(msg string, fields ...log.Field) { /* ... */ }
func (l *MyZapLogger) Info(msg string, fields ...log.Field) { /* ... */ }
func (l *MyZapLogger) Warn(msg string, fields ...log.Field) { /* ... */ }
func (l *MyZapLogger) Error(msg string, fields ...log.Field) { /* ... */ }
```

## Version Checking

Each module now exposes version constants:

```go
import (
    "github.com/bft-labs/walship/pkg/walship"
    "github.com/bft-labs/walship/pkg/wal"
)

// Check versions
fmt.Println(walship.Version)          // "1.0.0"
fmt.Println(wal.Version)              // "1.0.0"

// Get all module versions
versions := walship.ModuleVersions()
for module, version := range versions {
    fmt.Printf("%s: %s\n", module, version)
}
```

## Dependency Changes

The modular architecture has no additional dependencies. Each module only imports:
- Standard library
- Other walship modules (as needed)

No external dependencies are required for core functionality.

## Questions?

- See `pkg/walship/doc.go` for complete API documentation
- See `pkg/*/doc.go` for individual module documentation
- File issues at https://github.com/bft-labs/walship/issues
