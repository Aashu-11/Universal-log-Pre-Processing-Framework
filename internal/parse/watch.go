package parse

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// WatchDir loads dir once, then watches it for writes/creates/renames and
// re-publishes the affected parser on every change — this is what makes
// "parsers are data, hot-swapped at runtime" literal: touching a .yaml file
// under packs/ takes effect within one filesystem-event round trip, no
// restart. OnReload, if non-nil, is called after every successful publish
// (used by the demo/tests to observe reload latency); OnError is called on
// any load/publish failure, which never removes the previously-published
// working Plan.
func (r *Registry) WatchDir(ctx context.Context, dir string, onReload func(id string), onErr func(error)) error {
	if _, err := r.LoadDir(dir); err != nil {
		return fmt.Errorf("initial load: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer watcher.Close()

	if err := addRecursive(watcher, dir); err != nil {
		return fmt.Errorf("watch %s: %w", dir, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !isParserArtifact(ev.Name) {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			p, err := LoadParserFile(ev.Name)
			if err != nil {
				if onErr != nil {
					onErr(fmt.Errorf("reload %s: %w", ev.Name, err))
				}
				continue
			}
			if _, err := r.Publish(p); err != nil {
				if onErr != nil {
					onErr(fmt.Errorf("publish %s: %w", ev.Name, err))
				}
				continue
			}
			if onReload != nil {
				onReload(p.Metadata.ID)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			if onErr != nil {
				onErr(err)
			}
		}
	}
}

func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return w.Add(path)
		}
		return nil
	})
}
