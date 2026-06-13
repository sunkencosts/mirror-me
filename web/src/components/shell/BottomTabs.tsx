import { useNavigate } from "react-router";
import { useCurrentLeague } from "../../hooks/useCurrentLeague";
import { Icon, type IconName } from "../icons";

const TABS: { key: string; segment: string; icon: IconName; label: string }[] = [
	{ key: "lineups", segment: "lineups", icon: "stack", label: "Lineups" },
	{ key: "stats", segment: "stats", icon: "chart", label: "Stats" },
	{ key: "setters", segment: "best-setters", icon: "medal", label: "Setters" },
];

interface BottomTabsProps {
	onOpenDrawer: () => void;
}

export default function BottomTabs({ onOpenDrawer }: BottomTabsProps) {
	const navigate = useNavigate();
	const { leagueId, routeKey } = useCurrentLeague();
	const moreActive = routeKey !== "home" && !TABS.some((t) => t.key === routeKey);

	return (
		<nav className="bottom-tabs">
			{TABS.map((tab) => (
				<button
					key={tab.key}
					type="button"
					className={routeKey === tab.key ? "active" : ""}
					onClick={() => navigate(leagueId ? `/${leagueId}/${tab.segment}` : "/")}
				>
					<Icon name={tab.icon} className="ic" />
					{tab.label}
				</button>
			))}
			<button type="button" className={moreActive ? "active" : ""} onClick={onOpenDrawer}>
				<Icon name="ellipsis" className="ic" />
				More
			</button>
		</nav>
	);
}
