import { useNavigate } from "react-router";
import { useCurrentLeague } from "../../hooks/useCurrentLeague";
import { Icon } from "../icons";

interface BottomTabsProps {
	onOpenDrawer: () => void;
}

export default function BottomTabs({ onOpenDrawer }: BottomTabsProps) {
	const navigate = useNavigate();
	const { leagueId, routeKey } = useCurrentLeague();
	const moreActive = routeKey !== "lineups" && routeKey !== "members";

	return (
		<nav className="bottom-tabs">
			<button
				type="button"
				className={routeKey === "members" ? "active" : ""}
				onClick={() => navigate(leagueId ? `/${leagueId}/teams` : "/")}
			>
				<Icon name="users" className="ic" />
				Teams
			</button>
			<button
				type="button"
				className={`fab${routeKey === "lineups" ? " active" : ""}`}
				onClick={() => navigate(leagueId ? `/${leagueId}/lineups` : "/")}
			>
				<span className="fab-circle">
					<Icon name="stack" className="ic" />
				</span>
				Lineups
			</button>
			<button type="button" className={moreActive ? "active" : ""} onClick={onOpenDrawer}>
				<Icon name="ellipsis" className="ic" />
				More
			</button>
		</nav>
	);
}
