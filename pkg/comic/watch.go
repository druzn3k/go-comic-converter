package comic

import (
	"context"
	"log"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

var supportedExts = map[string]bool{
	".cbz": true, ".zip": true, ".cbr": true, ".rar": true, ".pdf": true,
}

// Watch monitors a directory for new comic files and auto-converts them.
// Supported extensions: .cbz, .zip, .cbr, .rar, .pdf.
// Each new file is converted in a background goroutine using the provided
// options. The Output field of opts is ignored; each job writes to
// [input-basename].epub in the current working directory.
func Watch(ctx context.Context, dir string, opts Options) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(dir); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				ext := strings.ToLower(filepath.Ext(event.Name))
				if supportedExts[ext] {
					jobOpts := opts
					jobOpts.Input = event.Name
					base := strings.TrimSuffix(event.Name, filepath.Ext(event.Name))
					jobOpts.Output = base + ".epub"
					go func() {
						if err := New(jobOpts).Convert(ctx); err != nil {
							log.Printf("Watch: %s: %v", event.Name, err)
						}
					}()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}
