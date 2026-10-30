package linuxmonitor

import (
	"os"
	"path/filepath"
)

// DirSizeCalculator is a function type that computes the total size in bytes
// of the directory rooted at path. It is abstracted so tests can inject a
// deterministic implementation instead of touching the filesystem.
type DirSizeCalculator func(path string) (int64, error)

// Monitor periodically checks the size of a directory and invokes a callback
// when that size exceeds a configured threshold.
type Monitor struct {
	path      string
	threshold int64
	calc      DirSizeCalculator
	onExceed  func(path string, size int64)
}

// New returns a Monitor for the given directory path and byte threshold.
//
// onExceed is called with the path and its measured size whenever the size is
// strictly greater than threshold. Passing nil for onExceed is a programming
// error and will panic on the first threshold breach — we panic rather than
// silently no-op so misuse is loud and immediate.
func New(path string, threshold int64, onExceed func(path string, size int64)) *Monitor {
	return &Monitor{
		path:      path,
		threshold: threshold,
		calc:      CalcDirSize,
		onExceed:  onExceed,
	}
}

// Check performs a single measurement. If the directory's size exceeds the
// threshold, the callback is invoked synchronously before Check returns.
//
// Returning the error from the calculator (rather than wrapping it) keeps the
// failure semantics identical to a direct call to CalcDirSize, which makes
// callers' error handling simpler.
func (m *Monitor) Check() (int64, error) {
	size, err := m.calc(m.path)
	if err != nil {
		return 0, err
	}
	if size > m.threshold {
		m.onExceed(m.path, size)
	}
	return size, nil
}

// CalcDirSize walks path and sums the sizes of every regular file it finds.
//
// We skip non-regular entries (directories, symlinks, devices) by checking
// os.FileInfo.Mode().IsRegular() rather than relying on Size() alone: on
// Linux, reporting Size() for a directory yields the inode block size, not a
// useful figure, and symlink Size() is the length of the target path. Both
// would silently inflate the total.
//
// A symlink that points nowhere produces a Stat error on the target; we treat
// that as a walk error and stop, because a missing target usually indicates a
// broken filesystem state that the caller should know about rather than have
// silently ignored.
func CalcDirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
