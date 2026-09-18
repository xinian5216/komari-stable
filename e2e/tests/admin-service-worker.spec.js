import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";
import { expect, test } from "@playwright/test";

const here = path.dirname(fileURLToPath(import.meta.url));
const run = JSON.parse(fs.readFileSync(path.join(here, "..", ".run.json"), "utf8"));

const NEXT_HOME = /__next|_next\//;
const DEFAULT_ENTRY = /entry-index-/;
const ADMIN_LAYOUT = ".km-admin-layout, .km-admin-panel-bar";

async function login(page) {
  const response = await page.request.post("/api/login", {
    data: { username: run.adminUser, password: run.adminPassword },
  });
  expect(response.ok(), `login HTTP ${response.status()}`).toBeTruthy();
}

async function assertLoggedIn(page) {
  const me = await page.request.get("/api/me");
  expect(me.ok()).toBeTruthy();
  const payload = await me.json();
  const loggedIn = payload.logged_in ?? payload.data?.logged_in;
  expect(loggedIn, JSON.stringify(payload)).toBe(true);
}

async function assertNextHome(page) {
  await expect(page).toHaveURL(/\/(?:\?.*)?$/);
  const html = await page.content();
  expect(html).toMatch(NEXT_HOME);
  expect(html).not.toMatch(DEFAULT_ENTRY);
  await expect(page.locator("main").first()).toBeVisible();
}

async function assertAdmin(page, adminPath = "/admin/dashboard") {
  await expect(page).not.toHaveURL(/\/(?:\?.*)?$/);
  await expect(page).toHaveURL(new RegExp(`${adminPath.replaceAll("/", "\\/")}(?:\\?.*)?$`));
  const html = await page.content();
  expect(html).toMatch(DEFAULT_ENTRY);
  expect(html).not.toMatch(NEXT_HOME);
  await expect(page.locator(ADMIN_LAYOUT).first()).toBeVisible();
  await expect(page.locator("#root")).toBeVisible();
}

test.describe.configure({ mode: "serial" });

test("case 1: fresh install seeds theme=next", async ({ request }) => {
  const publicInfo = await request.get("/api/public");
  expect(publicInfo.ok()).toBeTruthy();
  const payload = await publicInfo.json();
  const theme = payload.theme ?? payload.data?.theme;
  expect(theme, JSON.stringify(payload)).toBe("next");

  const themeDir = path.join(run.instanceDir, "data", "theme", "next");
  expect(fs.existsSync(path.join(themeDir, "komari-theme.json"))).toBeTruthy();
  expect(fs.existsSync(path.join(themeDir, "dist", "index.html"))).toBeTruthy();
});

test("case 2: public homepage is Komari Next", async ({ page }) => {
  await page.goto("/");
  await assertNextHome(page);
});

test("case 3: logged-in /admin and /admin/dashboard stay on the embedded panel", async ({
  page,
}) => {
  await login(page);
  await assertLoggedIn(page);

  await page.goto("/admin");
  await page.waitForURL(/\/admin\/dashboard/);
  await assertAdmin(page);
  await assertLoggedIn(page);

  await page.reload();
  await assertAdmin(page);
  await assertLoggedIn(page);

  const other = await page.context().newPage();
  await other.goto("/admin/dashboard");
  await assertAdmin(other);
  await other.close();
});

test("case 4: service worker must not turn admin into the Next homepage", async ({ page }) => {
  await login(page);
  await page.goto("/");
  await assertNextHome(page);

  await page.goto("/admin/dashboard");
  await assertAdmin(page);

  await page.evaluate(async () => {
    if (!("serviceWorker" in navigator)) {
      return;
    }
    await navigator.serviceWorker.register("/sw.js", { scope: "/" });
    await navigator.serviceWorker.ready;
  });
  await page.waitForTimeout(1000);

  await page.reload();
  await assertAdmin(page);

  await page.goto("/");
  await assertNextHome(page);

  await page.goto("/admin/dashboard");
  await assertAdmin(page);
  await page.reload();
  await assertAdmin(page);
  await assertLoggedIn(page);
});

test("case 5: stale Workbox cache does not hijack admin after the upgraded SW", async ({
  page,
  request,
}) => {
  const sw = await request.get("/sw.js");
  expect(sw.ok()).toBeTruthy();
  const swBody = await sw.text();
  expect(swBody).toContain("komari-core-route-sw-bypass-v1");

  await login(page);
  await page.goto("/");
  await page.evaluate(async () => {
    await navigator.serviceWorker.register("/sw.js", { scope: "/" });
    await navigator.serviceWorker.ready;
    const staleHome = new Response(
      '<!DOCTYPE html><html><body><div id="__next">STALE_PRECACHED_HOME</div></body></html>',
      { headers: { "content-type": "text/html; charset=utf-8" } },
    );
    const staleAdmin = new Response(
      '<!DOCTYPE html><html><body><div id="__next">STALE_CACHED_ADMIN</div></body></html>',
      { headers: { "content-type": "text/html; charset=utf-8" } },
    );
    const extra = await caches.open("workbox-precache-v2-stale-komari");
    await extra.put("/index.html", staleHome.clone());
    await extra.put("/admin/dashboard", staleAdmin.clone());
    for (const key of await caches.keys()) {
      const cache = await caches.open(key);
      await cache.put("/index.html", staleHome.clone());
      await cache.put("/admin/dashboard", staleAdmin.clone());
    }
  });

  await page.goto("/admin/dashboard");
  await assertAdmin(page);
  const html = await page.content();
  expect(html).not.toContain("STALE_CACHED_ADMIN");
  expect(html).not.toContain("STALE_PRECACHED_HOME");
  await page.reload();
  await assertAdmin(page);
  await assertLoggedIn(page);
});
