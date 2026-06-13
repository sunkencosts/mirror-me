// Live verification: logs in via /dev/login and screenshots the REAL pages
// (My Leagues, Lineups, Members) against the running dev server, which proxies
// to the live Go backend + Postgres. No mock data — real Sleeper league.
//
// Requires: vite dev on :5173 and the Go API on :8080 (the dev.sh stack).
//   node scripts/verify-live.mjs [leagueId]

import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(here, "..", "parity-out");
mkdirSync(outDir, { recursive: true });

const base = process.env.APP_URL ?? "http://localhost:5173";
const leagueId = process.argv[2] ?? "1182073403987832832";
const DESKTOP = { width: 1320, height: 920 };

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: DESKTOP });
const page = await ctx.newPage();
const errors = [];
page.on("console", (m) => {
	if (m.type() === "error") {
		errors.push(m.text());
	}
});

// 1) dev login (sets the JWT cookie, redirects to the SPA)
await page.goto(`${base}/dev/login?username=dev_user`, { waitUntil: "networkidle" });

const shots = [
	["live-home", `${base}/`],
	["live-lineups", `${base}/${leagueId}/lineups`],
	["live-members", `${base}/${leagueId}/members`],
];

for (const [name, url] of shots) {
	await page.goto(url, { waitUntil: "networkidle" });
	// give Sleeper-backed queries time to resolve and render
	await page.waitForTimeout(2500);
	await page.screenshot({ path: resolve(outDir, `${name}.png`), fullPage: true });
	console.log(`shot ${name} <- ${url}`);
}

await browser.close();
if (errors.length) {
	console.log("\nconsole errors:\n" + errors.slice(0, 10).join("\n"));
} else {
	console.log("\nno console errors");
}
