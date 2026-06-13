// Visual parity harness: screenshots the design prototype and our running app
// for each shell screen at desktop + mobile, and computes a focused pixel diff
// on the sidebar chrome region (where the two SHOULD match this increment).
//
// Usage:
//   node scripts/parity.mjs            # app at http://localhost:4173
//   APP_URL=http://localhost:5173 node scripts/parity.mjs
//
// Note: the prototype ships with mock data while a no-backend app run shows the
// guest/empty state, so full-page diffs are dominated by *content*, not shell
// fidelity. Review the *-app/-proto full screenshots by eye; the sidebar-clip
// diff number is the closest automated signal for shell structure/styling.

import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import pixelmatch from "pixelmatch";
import { chromium } from "playwright";
import { PNG } from "pngjs";

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(here, "..");
const repoRoot = resolve(webRoot, "..");
const protoFile = resolve(repoRoot, "design_handoff_mirror_league", "Mirror League.html");
const protoUrl = `file://${encodeURI(protoFile)}`;
const appUrl = process.env.APP_URL ?? "http://localhost:4173";
const outDir = resolve(webRoot, "parity-out");
mkdirSync(outDir, { recursive: true });

const DESKTOP = { width: 1320, height: 920 };
const MOBILE = { width: 390, height: 844 };

// screen key -> { proto route param, app path }
const SCREENS = [
	{ key: "home", proto: "home", app: "/" },
	{ key: "leaderboard", proto: "leaderboard", app: "/leaderboard" },
	{ key: "rookies", proto: "rookies", app: "/rankings/rookies" },
];

function protoHref(route, { drawer = false } = {}) {
	const params = new URLSearchParams({ route, chrome: "off" });
	if (drawer) {
		params.set("drawer", "1");
	}
	return `${protoUrl}?${params.toString()}`;
}

async function shoot(page, url, viewport, file) {
	await page.setViewportSize(viewport);
	await page.goto(url, { waitUntil: "networkidle" });
	await page.waitForTimeout(400);
	await page.screenshot({ path: file });
}

function sidebarDiff(protoPng, appPng, width) {
	const a = PNG.sync.read(readFileSync(protoPng));
	const b = PNG.sync.read(readFileSync(appPng));
	const h = Math.min(a.height, b.height);
	const w = Math.min(width, a.width, b.width);
	const clip = (src) => {
		const out = new PNG({ width: w, height: h });
		for (let y = 0; y < h; y++) {
			for (let x = 0; x < w; x++) {
				const si = (src.width * y + x) << 2;
				const di = (w * y + x) << 2;
				out.data[di] = src.data[si];
				out.data[di + 1] = src.data[si + 1];
				out.data[di + 2] = src.data[si + 2];
				out.data[di + 3] = src.data[si + 3];
			}
		}
		return out;
	};
	const ca = clip(a);
	const cb = clip(b);
	const diff = new PNG({ width: w, height: h });
	const mismatched = pixelmatch(ca.data, cb.data, diff.data, w, h, { threshold: 0.1 });
	return { mismatched, total: w * h, pct: (100 * mismatched) / (w * h) };
}

const browser = await chromium.launch();
const page = await browser.newPage();
const report = [];

for (const screen of SCREENS) {
	// desktop
	const pProto = resolve(outDir, `${screen.key}-desktop-proto.png`);
	const pApp = resolve(outDir, `${screen.key}-desktop-app.png`);
	await shoot(page, protoHref(screen.proto), DESKTOP, pProto);
	await shoot(page, `${appUrl}${screen.app}`, DESKTOP, pApp);
	const d = sidebarDiff(pProto, pApp, 256);
	report.push(`${screen.key.padEnd(12)} desktop  sidebar-diff ${d.pct.toFixed(1)}%`);

	// mobile (drawer open shows the sidebar)
	await shoot(
		page,
		protoHref(screen.proto, { drawer: true }),
		MOBILE,
		resolve(outDir, `${screen.key}-mobile-proto.png`),
	);
	await shoot(
		page,
		`${appUrl}${screen.app}`,
		MOBILE,
		resolve(outDir, `${screen.key}-mobile-app.png`),
	);
}

await browser.close();

// quick sanity: report whether chromium rendered Archivo (font may be offline)
console.log("\nParity screenshots written to parity-out/\n");
console.log(report.join("\n"));
