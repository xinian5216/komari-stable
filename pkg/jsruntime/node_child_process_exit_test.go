package jsruntime

import (
	"io"
	"os"
	"testing"
	"time"
)

// TestChildProcessOutputSurvivesProcessExit guards the child process reaper.
//
// exec.Cmd.Wait closes the pipes returned by StdoutPipe/StderrPipe as soon as it
// sees the command exit, so calling it while the stdout/stderr pumps are still
// reading drops whatever the child already wrote and can stop the streams from
// ever seeing EOF. The JS side then either observes an empty string at the
// "close" event or waits for "end" until the runtime times out, which is what
// made the child_process tests fail intermittently on Linux.
//
// The child here is this test binary itself running the helper in "output"
// mode, which writes 4096 bytes and exits immediately - the smallest case that
// still races with the reaper.
func TestChildProcessOutputSurvivesProcessExit(t *testing.T) {
	jsRuntime, err := New(`
		const childProcess = require("child_process");
		async function verify(command) {
			const runs = [];
			for (let index = 0; index < 16; index++) {
				const child = childProcess.spawn(command, ["-test.run=^TestNodeChildProcessHelper$"], {
					env: Object.assign({}, process.env, { KOMARI_JSRUNTIME_CHILD_HELPER_MODE: "output" }),
				});
				let output = "";
				child.stdout.setEncoding("utf8");
				child.stdout.on("data", (chunk) => output += chunk);
				const ended = new Promise((resolve) => child.stdout.on("end", resolve));
				const code = await new Promise((resolve) => child.on("close", resolve));
				await ended;
				runs.push({ code: code, size: output.length });
			}
			if (runs.every((run) => run.code === 0 && run.size === 4096)) {
				return "all output received before close";
			}
			throw new Error("output was lost or truncated: " + JSON.stringify(runs));
		}
	`, Options{NodeJS: true, AllowExec: true, BaseDir: t.TempDir(), Console: io.Discard, Timeout: 60 * time.Second})
	if err != nil {
		t.Fatalf("runtime init: %v", err)
	}
	defer jsRuntime.Close()

	if err := jsRuntime.Call("verify", os.Args[0]); err != nil {
		t.Fatalf("child process output handling: %v", err)
	}
}
