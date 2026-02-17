package output

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/efan/proxyyopick/internal/model"
	"github.com/efan/proxyyopick/internal/tester"
)

// TxtWriter writes successful proxies as ip:port lines to a text file.
type TxtWriter struct {
	Dir string
}

func NewTxtWriter(dir string) *TxtWriter {
	return &TxtWriter{Dir: dir}
}

func (w *TxtWriter) Write(results []model.TestResult) error {
	if err := os.MkdirAll(w.Dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	path := filepath.Join(w.Dir, "proxies.txt")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	successful := tester.FilterSuccessful(results)
	for _, r := range successful {
		fmt.Fprintf(f, "%s:%d\n", r.Proxy.IP, r.Proxy.Port)
	}

	fmt.Printf("📄 TXT saved: %s (%d proxies)\n", path, len(successful))
	return nil
}
