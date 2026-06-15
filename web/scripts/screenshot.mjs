// Quick page screenshot: launch headless chromium, navigate, screenshot, report console errors.
//
//   node scripts/screenshot.mjs <path> [out-name]
//
// <path> is appended to APP_URL (default http://localhost:5173), e.g.:
//   node scripts/screenshot.mjs /rankings/rookies rookies
//
// For pages that require auth, set DEV_LOGIN to log in via /dev/login first
// (only available when the API runs with APP_ENV != production):
//   DEV_LOGIN=dev_user node scripts/screenshot.mjs /1182073403987832832/lineups lineups

import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(here, "..", "parity-out");
mkdirSync(outDir, { recursive: true });

const base = process.env.APP_URL ?? "http://localhost:5173";
const path = process.argv[2] ?? "/";
const name = process.argv[3] ?? path.replace(/^\/|\/$/g, "").replace(/\//g, "-") ?? "home";
const outName = name === "" ? "home" : name;

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1320, height: 920 } });
const page = await ctx.newPage();
const errors = [];
page.on("console", (m) => {
	if (m.type() === "error") {
		errors.push(m.text());
	}
});

const devLoginUser = process.env.DEV_LOGIN;
if (devLoginUser) {
	await page.goto(`${base}/dev/login?username=${encodeURIComponent(devLoginUser)}`, {
		waitUntil: "load",
	});
}

// "networkidle" never resolves with Vite's dev server (HMR websocket stays open).
await page.goto(`${base}${path}`, { waitUntil: "load" });
await page.waitForTimeout(1500);

const outPath = resolve(outDir, `${outName}.png`);
await page.screenshot({ path: outPath, fullPage: true });
console.log(`shot ${outName} <- ${base}${path}`);
console.log(`saved to ${outPath}`);

await browser.close();
if (errors.length) {
	console.log("\nconsole errors:\n" + errors.slice(0, 10).join("\n"));
} else {
	console.log("\nno console errors");
}
