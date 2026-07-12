import { useState } from "react";
import { useCurrentLeague } from "../../hooks/useCurrentLeague";
import { useLeagueName } from "../../hooks/useLeagueName";
import type { AuthUser } from "../../types";
import { avatarBg, initials } from "../../utils/avatar";
import { Icon } from "../icons";
import AccountMenu from "./AccountMenu";
import { routeByKey } from "./routes";

interface MobileAppBarProps {
	user: AuthUser | null;
	isLoading: boolean;
	onOpenDrawer: () => void;
	onLogout: () => void;
}

export default function MobileAppBar({
	user,
	isLoading,
	onOpenDrawer,
	onLogout,
}: MobileAppBarProps) {
	const { routeKey } = useCurrentLeague();
	const leagueName = useLeagueName();
	const route = routeByKey(routeKey);
	const title = route?.title ?? "Mirror League";
	const sub = route?.scope === "league" ? (leagueName ?? "League") : "Global";
	const [open, setOpen] = useState(false);
	const name = user?.display_name ?? "guest";

	function handleAvatarClick() {
		if (user) {
			setOpen((o) => !o);
		} else {
			window.location.href = `${import.meta.env.VITE_API_URL ?? ""}/auth/google`;
		}
	}

	return (
		<div className="mobile-bar">
			<button type="button" className="ham" aria-label="Open menu" onClick={onOpenDrawer}>
				<Icon name="menu" />
			</button>
			<div className="midcol">
				<div className="mtitle">{title}</div>
				<div className="msub">{sub}</div>
			</div>
			<span className="spacer" />
			<div className="account-anchor">
				<button
					type="button"
					className="mav pav r-myt"
					style={{ background: isLoading ? undefined : avatarBg(name), fontSize: 12 }}
					onClick={handleAvatarClick}
					aria-label="Account"
					aria-expanded={user ? open : undefined}
					aria-haspopup={user ? "menu" : undefined}
					disabled={isLoading}
				>
					{isLoading ? "" : user ? initials(user.display_name) : "?"}
				</button>
				{open && user && (
					<AccountMenu
						user={user}
						onLogout={onLogout}
						onClose={() => setOpen(false)}
						variant="appbar"
					/>
				)}
			</div>
		</div>
	);
}
