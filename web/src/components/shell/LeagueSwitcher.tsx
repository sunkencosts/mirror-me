import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router";
import { bookmarksKey, fetchJson } from "../../api";
import { useAuth } from "../../context/AuthContext";
import type { LeagueBookmark } from "../../types";
import { Icon } from "../icons";

/** League avatar in the switcher; falls back to the ⛳ badge when missing or on load error. */
function LeagueIcon({ iconUrl }: { iconUrl?: string }) {
	const [failed, setFailed] = useState(false);
	return (
		<div className="lg-av">
			{iconUrl && !failed ? (
				<img src={iconUrl} alt="" onError={() => setFailed(true)} />
			) : (
				"⛳"
			)}
		</div>
	);
}

interface LeagueSwitcherProps {
	leagueId: string | null;
	leagueName: string | null;
	subLabel?: string;
	onNavigate?: () => void;
}

export default function LeagueSwitcher({
	leagueId,
	leagueName,
	subLabel,
	onNavigate,
}: LeagueSwitcherProps) {
	const { userId } = useAuth();
	const [open, setOpen] = useState(false);

	const { data: bookmarks = [] } = useQuery<LeagueBookmark[]>({
		queryKey: bookmarksKey(userId ?? ""),
		queryFn: () => fetchJson(`/league-bookmarks?user_id=${userId}`),
		enabled: !!userId,
	});

	const selected = bookmarks.find((b) => b.league_id === leagueId);

	function close() {
		setOpen(false);
	}

	function handleNavigate() {
		close();
		onNavigate?.();
	}

	return (
		<div className="switcher-wrap">
			<button
				type="button"
				className="switcher"
				onClick={() => setOpen((o) => !o)}
				aria-expanded={open}
			>
				<LeagueIcon iconUrl={selected?.icon_url} />
				<div className="lg-name">
					{leagueId ? leagueName || "League" : "Connect a league"}
					<div className="lg-sub">
						{leagueId ? subLabel ?? "Open lineups" : "No league selected"}
					</div>
				</div>
				<Icon name="chevDown" className="chev" />
			</button>
			{open && (
				<div className="pop">
					<div className="pop-list">
						{bookmarks.length > 0 ? (
							bookmarks.map((b) => (
								<Link
									key={b.league_id}
									to={`/${b.league_id}/lineups`}
									className={`pop-item${b.league_id === leagueId ? " active" : ""}`}
									onClick={handleNavigate}
								>
									<LeagueIcon iconUrl={b.icon_url} />
									<div style={{ flex: 1, minWidth: 0 }}>
										<div className="pname">{b.label || b.league_id}</div>
										<div className="pmeta">Open lineups</div>
									</div>
									<Icon name="check" className="pick" />
								</Link>
							))
						) : (
							<div className="pop-empty">No leagues yet — connect one to get started.</div>
						)}
					</div>
					<Link to="/" className="pop-item pop-footer" onClick={handleNavigate}>
						<Icon name="grid" />
						<div className="pname">My Leagues</div>
					</Link>
				</div>
			)}
			{open && (
				<button type="button" aria-label="Close" className="pop-backdrop" onClick={close} />
			)}
		</div>
	);
}
