import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [react()],
	server: {
		proxy: {
			"/auth": "http://localhost:8080",
			"/dev": "http://localhost:8080",
			"/league": {
				target: "http://localhost:8080",
				// /league/:leagueId(/week/:week) is also a frontend page route, so let
				// browser navigations (Accept: text/html) fall through to the SPA
				// instead of hitting the API and getting raw JSON.
				bypass: (req) => {
					if (req.headers.accept?.includes("text/html")) {
						return "/index.html";
					}
				},
			},
			"/lineups": "http://localhost:8080",
			"/players": "http://localhost:8080",
			"/league-bookmarks": "http://localhost:8080",
			"/admin": "http://localhost:8080",
		},
	},
});
