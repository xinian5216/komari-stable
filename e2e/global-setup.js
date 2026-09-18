import fs from "fs";
import os from "os";
import path from "path";
import { spawn } from "child_process";
import { fileURLToPath } from "url";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "..");
const runFile = path.join(here, ".run.json");
const listenHost = "127.0.0.1";
const listenPort = process.env.KOMARI_E2E_PORT || "25775";
const baseURL = `http://${listenHost}:${listenPort}`;
const adminUser = "e2eadmin";
const adminPassword = "E2eAdmin#2026";

function findBinary() {
  if (process.env.KOMARI_BIN) {
    return process.env.KOMARI_BIN;
  }
  const names = process.platform === "win32" ? ["komari.exe", "komari"] : ["komari"];
  for (const name of names) {
    const candidate = path.join(repoRoot, name);
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }
  return null;
}

async function waitFor(url, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let lastError = "";
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url, { redirect: "manual" });
      if (response.ok || (response.status >= 300 && response.status < 500)) {
        return;
      }
      lastError = `HTTP ${response.status}`;
    } catch (error) {
      lastError = error instanceof Error ? error.message : String(error);
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`timeout waiting for ${url}: ${lastError}`);
}

export default async function globalSetup() {
  const bin = findBinary();
  if (!bin) {
    throw new Error(
      "komari binary not found. Build it first (go build -o komari .) or set KOMARI_BIN.",
    );
  }

  const instanceDir = fs.mkdtempSync(path.join(os.tmpdir(), "komari-admin-sw-e2e-"));
  const logFile = path.join(instanceDir, "server.log");
  const logFd = fs.openSync(logFile, "w");
  const child = spawn(bin, ["server", "-l", `${listenHost}:${listenPort}`], {
    cwd: instanceDir,
    stdio: ["ignore", logFd, logFd],
    windowsHide: true,
  });
  if (!child.pid) {
    throw new Error("failed to start komari server");
  }

  fs.writeFileSync(
    runFile,
    JSON.stringify(
      {
        baseURL,
        instanceDir,
        pid: child.pid,
        logFile,
        adminUser,
        adminPassword,
      },
      null,
      2,
    ) + "\n",
  );

  await waitFor(`${baseURL}/api/install/status`, 60000);
  const install = await fetch(`${baseURL}/api/install/complete`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      username: adminUser,
      password: adminPassword,
      sitename: "Admin SW E2E",
      description: "",
      metric_dsn: "./data/metrics.db",
    }),
  });
  if (!install.ok) {
    const body = await install.text();
    throw new Error(`fresh install failed: HTTP ${install.status} ${body}`);
  }

  await waitFor(`${baseURL}/api/version`, 60000);
  process.env.KOMARI_BASE_URL = baseURL;
}
