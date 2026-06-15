// Site crawl: logs in (as dev_user, or a real user via DEV_USER_ID), then
// visits every page in the app (global routes + every bookmarked league's
// routes) and reports anything that looks off — console errors, uncaught
// exceptions, failed requests, broken images, or an error-boundary screen.
//
// Requires the dev.sh stack (vite on :5173, Go API on :8080).
//   node scripts/crawl.mjs
//   DEV_USER_ID=<uuid> node scripts/crawl.mjs   # crawl a real user's leagues

import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(here, "..", "parity-out", "crawl");
mkdirSync(outDir, { recursive: true });

const base = process.env.APP_URL ?? "http://localhost:5173";
const userId = process.env.DEV_USER_ID ?? "00000000-0000-0000-0000-000000000001";

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1320, height: 920 } });
const page = await ctx.newPage();

await page.goto(`${base}/dev/login?user_id=${userId}&username=dev_user`, { waitUntil: "load" });

const bookmarks = await page.evaluate(async (userId) => {
	const r = await fetch(`/league-bookmarks?user_id=${userId}`, { credentials: "include" });
	return r.ok ? r.json() : [];
}, userId);

const pages = [
	["home", "/"],
	["leaderboard", "/leaderboard"],
	["rookies", "/rankings/rookies"],
];
for (const b of bookmarks) {
	const id = b.league_id;
	for (const [seg, key] of [
		["lineups", "lineups"],
		["teams", "teams"],
		["stats", "stats"],
		["best-setters", "best-setters"],
	]) {
		pages.push([`${id}-${key}`, `/${id}/${seg}`]);
	}
}

const report = [];

for (const [name, path] of pages) {
	const consoleErrors = [];
	const pageErrors = [];
	const failedRequests = [];

	const onConsole = (m) => {
		if (m.type() === "error") consoleErrors.push(m.text());
	};
	const onPageError = (e) => pageErrors.push(e.message);
	const onResponse = (res) => {
		if (res.status() >= 400) failedRequests.push(`${res.status()} ${res.url()}`);
	};

	page.on("console", onConsole);
	page.on("pageerror", onPageError);
	page.on("response", onResponse);

	await page.goto(`${base}${path}`, { waitUntil: "load" });
	await page.waitForTimeout(2000);

	const bodyText = await page.evaluate(() => document.body.innerText);
	const errorBoundary = bodyText.includes("Back to home")
		? bodyText.split("\n").find((l) => l.trim()) ?? null
		: null;

	const brokenImages = await page.evaluate(() =>
		Array.from(document.querySelectorAll("img"))
			.filter((img) => img.src && img.naturalWidth === 0)
			.map((img) => img.src),
	);

	await page.screenshot({ path: resolve(outDir, `${name}.png`), fullPage: true });

	page.off("console", onConsole);
	page.off("pageerror", onPageError);
	page.off("response", onResponse);

	report.push({ name, path, consoleErrors, pageErrors, failedRequests, errorBoundary, brokenImages });
}

await browser.close();

console.log(`crawled ${report.length} pages, screenshots in ${outDir}\n`);
for (const r of report) {
	const issues =
		!!r.errorBoundary ||
		r.consoleErrors.length > 0 ||
		r.pageErrors.length > 0 ||
		r.failedRequests.length > 0 ||
		r.brokenImages.length > 0;
	console.log(`${issues ? "ISSUES" : "ok    "}  ${r.path}  (${r.name})`);
	if (r.errorBoundary) console.log(`  error boundary: ${r.errorBoundary}`);
	for (const e of r.consoleErrors) console.log(`  console error: ${e}`);
	for (const e of r.pageErrors) console.log(`  page error: ${e}`);
	for (const e of r.failedRequests) console.log(`  failed request: ${e}`);
	if (r.brokenImages.length) console.log(`  broken images: ${r.brokenImages.length}`);
}
