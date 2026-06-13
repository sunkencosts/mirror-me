import "./index.css";
import "./shell.css";
import "./lineups.css";
import "./home.css";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Agentation } from "agentation";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router";
import { router } from "./App.tsx";
import { AuthProvider } from "./context/AuthContext.tsx";
import { ApiError } from "./api.ts";

const queryClient = new QueryClient({
	defaultOptions: {
		queries: {
			retry: (count, error) => {
				if (error instanceof ApiError && error.status < 500) return false;
				return count < 3;
			},
		},
	},
});

const rootEl = document.getElementById("root");
if (!rootEl) {
	throw new Error("Root element not found");
}

createRoot(rootEl).render(
	<StrictMode>
		<QueryClientProvider client={queryClient}>
			<AuthProvider>
				<RouterProvider router={router} />
			</AuthProvider>
		</QueryClientProvider>
		{import.meta.env.DEV && <Agentation endpoint="http://localhost:4747" />}
	</StrictMode>,
);
