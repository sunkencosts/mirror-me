import { Link } from "react-router";
import { useCurrentLeague } from "../../hooks/useCurrentLeague";
import { useLeagueName } from "../../hooks/useLeagueName";
import type { AuthUser } from "../../types";
import { Icon } from "../icons";
import LeagueSwitcher from "./LeagueSwitcher";
import NavItem from "./NavItem";
import { GLOBAL_ROUTES, LEAGUE_ROUTES } from "./routes";
import UserChip from "./UserChip";

interface SidebarProps {
	user: AuthUser | null;
	isLoading: boolean;
	onLogout: () => void;
	open: boolean;
	onNavigate: () => void;
}

export default function Sidebar({ user, isLoading, onLogout, open, onNavigate }: SidebarProps) {
	const { leagueId, routeKey } = useCurrentLeague();
	const leagueName = useLeagueName();
	const hasLeague = leagueId !== null;

	return (
		<aside className={`side${open ? " open" : ""}`}>
			<Link to="/" className="brand" onClick={onNavigate}>
				<div className="mark">
					<Icon name="stack" />
				</div>
				<div className="wm">
					Mirror League
					<small>Mirror a Sleeper league</small>
				</div>
			</Link>

			<div className="nav-label">Global</div>
			{GLOBAL_ROUTES.map((r) => (
				<NavItem
					key={r.key}
					to={r.to()}
					icon={r.navIcon}
					label={r.navLabel}
					active={routeKey === r.key}
					onNavigate={onNavigate}
				/>
			))}

			<div className="side-div" />
			<div className="nav-label">League</div>
			<LeagueSwitcher leagueId={leagueId} leagueName={leagueName} onNavigate={onNavigate} />
			<div style={{ height: 8 }} />
			{LEAGUE_ROUTES.map((r) => (
				<NavItem
					key={r.key}
					to={r.to(leagueId ?? "")}
					icon={r.navIcon}
					label={r.navLabel}
					active={routeKey === r.key}
					disabled={!hasLeague}
					onNavigate={onNavigate}
				/>
			))}

			<UserChip user={user} isLoading={isLoading} onLogout={onLogout} />
		</aside>
	);
}
