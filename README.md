# linux-monitor

Periodically checks a directory's total size and invokes a callback when that size exceeds a configured byte threshold. Standard library only, no third-party dependencies.

```go
package main

import (
    "log"
    "linux-monitor"
)

func main() {
    m := linuxmonitor.New("/var/log", 1<<30, func(path string, size int64) {
        log.Printf("%s is %d bytes, over the 1 GiB limit", path, size)
    })
    if _, err := m.Check(); err != nil {
        log.Fatal(err)
    }
}
```

## Why this exists

Log directories, scratch space, and cache folders grow without bound. A monitor that calls back the instant a directory crosses a threshold lets a process trim, rotate, or alert before the volume fills. The library is intentionally thin — it gives you the measurement and the comparison, and leaves the response policy to the caller.

## API

- `New(path string, threshold int64, onExceed func(path string, size int64)) *Monitor` — construct a monitor. `onExceed` must not be nil; it is called synchronously from `Check` whenever the measured size is **strictly greater than** `threshold`.
- `(*Monitor).Check() (int64, error)` — measure once. Returns the size in bytes and any filesystem error. If the size exceeds the threshold, `onExceed` is called before `Check` returns.
- `CalcDirSize(path string) (int64, error)` — the default size calculator, also exported so you can call it directly. Walks the directory tree and sums every regular file's size.
- `DirSizeCalculator func(path string) (int64, error)` — the function type, exported so tests or alternate backends can substitute their own implementation by assigning to `Monitor.calc`.

## Edge cases worth knowing

The comparison is strictly-greater-than: a directory whose size equals the threshold exactly does **not** fire the callback. If you need >= semantics, subtract one from the threshold you pass in.

`CalcDirSize` only counts regular files. Symlinks are skipped, even when they point to a file or directory whose contents you might expect to be included. This is deliberate: following symlinks during a recursive walk risks double-counting and, worse, infinite loops on circular links. If you need symlinked-directory contents counted, resolve the real path before constructing the monitor.

The `onExceed` callback runs synchronously on the goroutine that calls `Check`. If your callback blocks or does heavy work, do that work elsewhere — the next `Check` will not start until the callback returns.
