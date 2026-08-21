package stage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// MarkerFileName is written at the root of a staged model once the copy has
// finished. Its presence is what distinguishes a complete stage from a
// directory left behind by an interrupted one — the agent only stats the model
// path, so without this an aborted copy would be served as if it were whole.
const MarkerFileName = ".ome-stage-complete"

// Marker records what a completed stage contains.
type Marker struct {
	SourcePath  string    `json:"sourcePath"`
	TotalBytes  int64     `json:"totalBytes"`
	CompletedAt time.Time `json:"completedAt"`
}

// Options configures a staging run.
type Options struct {
	// SourceRoots limits which paths may be staged. Empty means staging is off.
	SourceRoots []string
	// AlwaysCopy re-copies even when a completed stage is already present.
	// It carries DownloadPolicy=AlwaysDownload.
	AlwaysCopy bool
}

// Result describes what a staging run did.
type Result struct {
	// Copied is false when an existing completed stage was reused.
	Copied bool
	// BytesCopied counts regular file bytes written; zero when reusing.
	BytesCopied int64
	// SourcePath is the symlink-resolved source that was staged.
	SourcePath string
}

// Run copies the model at sourcePath onto the node's local disk at destPath.
//
// The copy lands in a sibling staging directory and is renamed into place only
// once it is complete, so destPath either does not exist or is a whole model.
func Run(sourcePath, destPath string, opts Options) (*Result, error) {
	resolvedSource, err := ResolveSource(sourcePath, opts.SourceRoots)
	if err != nil {
		return nil, err
	}

	// Staging into the source would copy the tree into itself. The comparison
	// needs both sides symlink-resolved, and the destination usually does not
	// exist yet, hence resolveExistingAncestor.
	resolvedDest, err := resolveExistingAncestor(destPath)
	if err != nil {
		return nil, err
	}
	if contains(resolvedSource, resolvedDest) {
		return nil, fmt.Errorf("stage destination %q must not be inside the source %q", destPath, sourcePath)
	}

	if !opts.AlwaysCopy {
		staged, err := IsStaged(destPath, resolvedSource)
		if err != nil {
			return nil, err
		}
		if staged {
			return &Result{Copied: false, SourcePath: resolvedSource}, nil
		}
	}

	destParent := filepath.Dir(destPath)
	if err := os.MkdirAll(destParent, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create stage destination parent %q: %w", destParent, err)
	}

	// Same directory as the destination, so the final rename stays within one
	// filesystem and is therefore atomic.
	stagingDir, err := os.MkdirTemp(destParent, filepath.Base(destPath)+".staging-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create staging directory under %q: %w", destParent, err)
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	bytesCopied, err := copyTree(resolvedSource, stagingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to stage %q: %w", sourcePath, err)
	}

	if err := writeMarker(stagingDir, Marker{
		SourcePath:  resolvedSource,
		TotalBytes:  bytesCopied,
		CompletedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}

	// A previous stage (or an interrupted one) has to go before the rename;
	// rename onto a non-empty directory fails.
	if err := os.RemoveAll(destPath); err != nil {
		return nil, fmt.Errorf("failed to clear stage destination %q: %w", destPath, err)
	}
	if err := os.Rename(stagingDir, destPath); err != nil {
		return nil, fmt.Errorf("failed to publish stage destination %q: %w", destPath, err)
	}
	succeeded = true

	return &Result{Copied: true, BytesCopied: bytesCopied, SourcePath: resolvedSource}, nil
}

// IsStaged reports whether destPath already holds a completed stage of
// sourcePath.
func IsStaged(destPath, sourcePath string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(destPath, MarkerFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read stage marker in %q: %w", destPath, err)
	}

	var marker Marker
	if err := json.Unmarshal(data, &marker); err != nil {
		// An unreadable marker is treated as no marker: re-staging is safe,
		// serving a possibly-truncated model is not.
		return false, nil
	}

	return marker.SourcePath == sourcePath, nil
}

// DirSize sums the size of every regular file under path.
func DirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(entry string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to size %q: %w", path, err)
	}
	return total, nil
}

// resolveExistingAncestor resolves symlinks in path, tolerating a path that
// does not exist yet: it resolves the deepest existing ancestor and re-appends
// the missing components.
func resolveExistingAncestor(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve stage destination %q: %w", path, err)
	}

	remainder := ""
	current := filepath.Clean(absolute)
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(resolved, remainder), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to resolve stage destination %q: %w", path, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached the filesystem root without finding anything that exists.
			return filepath.Join(current, remainder), nil
		}
		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
	}
}

func writeMarker(dir string, marker Marker) error {
	data, err := json.Marshal(marker)
	if err != nil {
		return fmt.Errorf("failed to encode stage marker: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, MarkerFileName), data, 0o644); err != nil {
		return fmt.Errorf("failed to write stage marker in %q: %w", dir, err)
	}
	return nil
}

// copyTree copies the contents of src into dst, which must already exist, and
// returns the number of regular file bytes written.
func copyTree(src, dst string) (int64, error) {
	var total int64

	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		switch {
		case d.IsDir():
			if rel == "." {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		case d.Type()&os.ModeSymlink != 0:
			// Copy the link itself; resolving it could pull in data from
			// outside the source roots.
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		case d.Type().IsRegular():
			n, err := copyFile(path, target, d)
			if err != nil {
				return err
			}
			total += n
			return nil
		default:
			// Sockets, devices and pipes have no place in a model directory.
			return nil
		}
	})
	if err != nil {
		return 0, err
	}

	return total, nil
}

func copyFile(src, dst string, entry os.DirEntry) (int64, error) {
	info, err := entry.Info()
	if err != nil {
		return 0, err
	}

	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return 0, err
	}

	written, err := io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return 0, err
	}

	return written, nil
}
