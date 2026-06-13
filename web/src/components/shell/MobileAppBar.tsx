import { useCurrentLeague } from "../../hooks/useCurrentLeague";
import { useLeagueName } from "../../hooks/useLeagueName";
import type { AuthUser } from "../../types";
import { avatarBg, initials } from "../../utils/avatar";
import { Icon } from "../icons";
import { routeByKey } from "./routes";

interface MobileAppBarProps {
	user: AuthUser | null;
	isLoading: boolean;
	onOpenDrawer: () => void;
}

export default function MobileAppBar({ user, isLoading, onOpenDrawer }: MobileAppBarProps) {
	const { routeKey } = useCurrentLeague();
	const leagueName = useLeagueName();
	const route = routeByKey(routeKey);
	const title = route?.title ?? "Mirror League";
	const sub = route?.scope === "league" ? (leagueName ?? "League") : "Global";
	const name = user?.username ?? "guest";

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
			<div
				className="mav pav r-myt"
				style={{ background: isLoading ? undefined : avatarBg(name), fontSize: 12 }}
			>
				{isLoading ? "" : user ? initials(user.username) : "?"}
			</div>
		</div>
	);
}
