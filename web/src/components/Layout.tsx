import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Outlet, ScrollRestoration, useLocation } from "react-router";
import { deleteJson } from "../api";
import { useAuth } from "../context/AuthContext";
import BottomTabs from "./shell/BottomTabs";
import MobileAppBar from "./shell/MobileAppBar";
import Sidebar from "./shell/Sidebar";
import Topbar from "./shell/Topbar";

export default function Layout() {
	const [drawerOpen, setDrawerOpen] = useState(false);
	const { user, isLoading } = useAuth();
	const queryClient = useQueryClient();
	const { pathname } = useLocation();

	// Close the mobile drawer whenever the route changes (incl. back/forward).
	// biome-ignore lint/correctness/useExhaustiveDependencies: pathname is the trigger
	useEffect(() => {
		setDrawerOpen(false);
	}, [pathname]);

	async function handleLogout() {
		await deleteJson("/auth/logout");
		queryClient.invalidateQueries({ queryKey: ["auth"] });
	}

	const closeDrawer = () => setDrawerOpen(false);
	const openDrawer = () => setDrawerOpen(true);

	return (
		<>
			<div className="bg-layer" />
			<div className="app">
				<button
					type="button"
					aria-label="Close menu"
					className={`scrim${drawerOpen ? " show" : ""}`}
					onClick={closeDrawer}
				/>
				<Sidebar
					user={user}
					isLoading={isLoading}
					onLogout={handleLogout}
					open={drawerOpen}
					onNavigate={closeDrawer}
				/>
				<div className="main">
					<MobileAppBar user={user} isLoading={isLoading} onOpenDrawer={openDrawer} />
					<Topbar />
					<main className="content">
						<Outlet />
					</main>
				</div>
				<BottomTabs onOpenDrawer={openDrawer} />
			</div>
			<ScrollRestoration />
		</>
	);
}
