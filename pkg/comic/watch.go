package comic

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"github.com/fsnotify/fsnotify"
)

var supportedExts = map[string]bool{
	".cbz": true, ".zip": true, ".cbr": true, ".rar": true, ".pdf": true,
}

// pendingJob holds a debounce timer and the conversion options for a
// file that has been seen but is still within its quiet window.
type pendingJob struct {
	timer *time.Timer
	opts  Options
}

// debounceDelay is how long the watcher waits after the last filesystem
// event for a file before starting the conversion.
const debounceDelay = 500 * time.Millisecond

// Watch monitors a directory for new comic files and auto-converts them.
// Supported extensions: .cbz, .zip, .cbr, .rar, .pdf.
// Rapid successive events for the same filename are coalesced via a
// debounce timer; temporary and hidden files are skipped.
func Watch(ctx context.Context, dir string, opts Options) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(dir); err != nil {
		return err
	}

	pending := make(map[string]*pendingJob)
	var mu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			mu.Lock()
			for _, p := range pending {
				p.timer.Stop()
			}
			mu.Unlock()
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(event.Name))
			if !supportedExts[ext] {
				continue
			}
			base := filepath.Base(event.Name)
			if isTempFile(base) {
				log.Printf("Watch: skipping temp file %s", event.Name)
				continue
			}

			mu.Lock()
			if p, exists := pending[event.Name]; exists {
				p.timer.Stop()
			}

			jobOpts := opts
			jobOpts.Input = event.Name
			baseName := strings.TrimSuffix(event.Name, filepath.Ext(event.Name))
			jobOpts.Output = baseName + ".epub"

			timer := time.AfterFunc(debounceDelay, func() {
				mu.Lock()
				delete(pending, event.Name)
				mu.Unlock()

				log.Printf("Watch: converting %s", event.Name)
				if err := New(jobOpts).Convert(ctx); err != nil {
					log.Printf("Watch: %s: %v", event.Name, err)
				} else {
					log.Printf("Watch: %s complete", event.Name)
				}
			})
			pending[event.Name] = &pendingJob{timer: timer, opts: jobOpts}
			mu.Unlock()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}

// isTempFile reports whether name looks like a temporary, swap, or hidden
// file that the watcher should ignore.
func isTempFile(name string) bool {
	return strings.HasPrefix(name, ".") ||
		strings.HasSuffix(name, "~") ||
		strings.HasSuffix(name, ".tmp") ||
		strings.HasSuffix(name, ".swp") ||
		strings.HasSuffix(name, ".swx") ||
		strings.Contains(name, ".goutputstream-")
}
