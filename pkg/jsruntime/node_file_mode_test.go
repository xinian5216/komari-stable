package jsruntime

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNodeFileCreationModeFromEncodingArgument guards the argument handling of
// the Node fs shim: Node accepts an encoding string where an options object is
// expected (fs.writeFileSync(file, data, "utf8")). Such a string is not a mode
// and must leave the default creation mode in place. Treating it as a mode
// yielded 0, which creates the file with mode 0000 on Unix (the very next read
// then fails with EACCES) and as a read-only file on Windows.
func TestNodeFileCreationModeFromEncodingArgument(t *testing.T) {
	baseDir := t.TempDir()
	jsRuntime, err := New(`
		const fs = require("fs");
		function write() {
			fs.writeFileSync("encoding.txt", "x", "utf8");
			fs.appendFileSync("append.txt", "x", "utf8");
			fs.writeFileSync("plain.txt", "x");
			fs.writeFileSync("object.txt", "x", { encoding: "utf8" });
			fs.writeFileSync("explicit.txt", "x", { mode: 0o600 });
			const fd = fs.openSync("opened.txt", "w", "utf8");
			fs.closeSync(fd);
		}
	`, Options{NodeJS: true, BaseDir: baseDir, Console: io.Discard, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("runtime init: %v", err)
	}
	defer jsRuntime.Close()

	if err := jsRuntime.CallVoid("write"); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	for _, name := range []string{"encoding.txt", "append.txt", "plain.txt", "object.txt", "explicit.txt", "opened.txt"} {
		path := filepath.Join(baseDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if perm := info.Mode().Perm(); perm&0o600 != 0o600 {
			t.Errorf("%s was created with mode %#o; an encoding string must not be read as a mode", name, perm)
		}
		if _, err := os.ReadFile(path); err != nil {
			t.Errorf("%s is not readable: %v", name, err)
		}
	}
}
