import fs from "fs";
import path from "path";
import { spawnSync } from "child_process";
import { fileURLToPath } from "url";

const here = path.dirname(fileURLToPath(import.meta.url));
const runFile = path.join(here, ".run.json");

export default async function globalTeardown() {
  if (!fs.existsSync(runFile)) {
    return;
  }
  const run = JSON.parse(fs.readFileSync(runFile, "utf8"));
  if (!run.pid) {
    return;
  }
  if (process.platform === "win32") {
    spawnSync("taskkill", ["/PID", String(run.pid), "/T", "/F"], {
      stdio: "ignore",
      windowsHide: true,
    });
    return;
  }
  try {
    process.kill(run.pid, "SIGTERM");
  } catch {
    // already exited
  }
}
