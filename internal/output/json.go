package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/efan/proxyyopick/internal/model"
)

// JSONWriter writes all results as a JSON file.
type JSONWriter struct {
	Dir string
}

func NewJSONWriter(dir string) *JSONWriter {
	return &JSONWriter{Dir: dir}
}

func (w *JSONWriter) Write(results []model.TestResult) error {
	if err := os.MkdirAll(w.Dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	path := filepath.Join(w.Dir, "proxies.json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	fmt.Printf("📋 JSON saved: %s (%d proxies)\n", path, len(results))
	return nil
}
