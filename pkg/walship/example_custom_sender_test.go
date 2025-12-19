package walship_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bft-labs/walship/pkg/batch"
	"github.com/bft-labs/walship/pkg/sender"
)

// FileSender demonstrates a custom sender implementation that writes batches to files.
// This shows that the Sender interface is implementation-agnostic.
type FileSender struct {
	outputDir string
	counter   int
}

// NewFileSender creates a new file-based sender.
func NewFileSender(outputDir string) *FileSender {
	return &FileSender{outputDir: outputDir}
}

// Send implements the sender.Sender interface by writing batches to JSON files.
func (f *FileSender) Send(ctx context.Context, b *batch.Batch, metadata sender.Metadata) error {
	// Create output directory if needed
	if err := os.MkdirAll(f.outputDir, 0755); err != nil {
		return err
	}

	// Create a serializable representation
	output := map[string]interface{}{
		"chain_id":    metadata.ChainID,
		"node_id":     metadata.NodeID,
		"frame_count": b.Size(),
		"byte_count":  b.TotalBytes,
		"frames":      b.Frames,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	// Write to numbered file
	f.counter++
	filename := filepath.Join(f.outputDir, "batch_"+string(rune('0'+f.counter))+".json")
	return os.WriteFile(filename, data, 0644)
}

// TestFileSenderImplementsSender verifies the FileSender implements the Sender interface.
func TestFileSenderImplementsSender(t *testing.T) {
	var _ sender.Sender = (*FileSender)(nil)
}

// ExampleFileSender demonstrates using a custom file-based sender.
func ExampleFileSender() {
	// Create a custom sender that writes to files instead of HTTP
	fileSender := NewFileSender("/tmp/walship-batches")

	// Create a batch using the proper constructor
	b := batch.NewBatch()
	// In real usage, you would add frames via b.Add(frame, compressed, idxLen)

	// Send using the custom implementation
	metadata := sender.Metadata{
		ChainID: "my-chain",
		NodeID:  "node-1",
	}

	_ = fileSender.Send(context.Background(), b, metadata)
	// Output files would be written to /tmp/walship-batches/batch_1.json
}
