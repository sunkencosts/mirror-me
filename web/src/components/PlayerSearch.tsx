import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { fetchJson } from "../api";
import { RARITY_ORDER, type Rarity } from "../rarity";
import type { Player, Roster, SlimPlayer } from "../types";
import { Icon } from "./icons";
import PlayerAvatar from "./PlayerAvatar";

function toPlayer(slim: SlimPlayer): Player {
	return { ...slim, number: 0, age: 0, active: true, rarity: slim.rarity as Rarity };
}

function rarityRank(r: string): number {
	const i = RARITY_ORDER.indexOf(r as Rarity);
	return i === -1 ? RARITY_ORDER.length : i;
}

interface Props {
	rosters: Roster[];
	onScrollToRoster: (rosterId: number) => void;
}

export default function PlayerSearch({ rosters, onScrollToRoster }: Props) {
	const [query, setQuery] = useState("");
	const [activeIndex, setActiveIndex] = useState<number | null>(null);
	const containerRef = useRef<HTMLDivElement>(null);

	const { data: allPlayers = [] } = useQuery<SlimPlayer[]>({
		queryKey: ["players"],
		queryFn: () => fetchJson("/players"),
		staleTime: Number.POSITIVE_INFINITY,
	});

	const ownerMap = useMemo(() => {
		const map = new Map<string, { rosterID: number; teamName: string }>();
		for (const roster of rosters) {
			for (const player of roster.players) {
				map.set(player.player_id, { rosterID: roster.roster_id, teamName: roster.team_name });
			}
		}
		return map;
	}, [rosters]);

	const results = useMemo(() => {
		if (!query) return [];
		const q = query.toLowerCase();
		return allPlayers
			.filter((p) => `${p.first_name} ${p.last_name}`.toLowerCase().includes(q))
			.map((p) => ({
				player: p,
				score:
					p.first_name.toLowerCase().startsWith(q) || p.last_name.toLowerCase().startsWith(q)
						? 0
						: 1,
			}))
			.sort(
				(a, b) => a.score - b.score || rarityRank(a.player.rarity) - rarityRank(b.player.rarity),
			)
			.slice(0, 8)
			.map(({ player }) => player);
	}, [allPlayers, query]);

	useEffect(() => {
		setActiveIndex(null);
	}, [results]);

	useEffect(() => {
		function handleClick(e: MouseEvent) {
			if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
				setQuery("");
			}
		}
		document.addEventListener("mousedown", handleClick);
		return () => document.removeEventListener("mousedown", handleClick);
	}, []);

	function handleSelect(player: SlimPlayer) {
		const owner = ownerMap.get(player.player_id);
		setQuery("");
		if (owner) {
			onScrollToRoster(owner.rosterID);
		}
	}

	function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
		if (results.length === 0) return;

		if (e.key === "ArrowDown") {
			e.preventDefault();
			setActiveIndex((i) => (i === null || i >= results.length - 1 ? 0 : i + 1));
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setActiveIndex((i) => (i === null || i <= 0 ? results.length - 1 : i - 1));
		} else if (e.key === "Enter" && activeIndex !== null) {
			e.preventDefault();
			handleSelect(results[activeIndex]);
		} else if (e.key === "Escape") {
			setQuery("");
		}
	}

	return (
		<div ref={containerRef} className="search-wrap">
			<div className="input">
				<Icon name="search" />
				<input
					value={query}
					onChange={(e) => setQuery(e.target.value)}
					onKeyDown={handleKeyDown}
					placeholder="Search players…"
					type="search"
					autoComplete="off"
				/>
			</div>
			{results.length > 0 && (
				<div className="search-pop">
					<div className="pop-list">
						{results.map((slim, i) => {
							const owner = ownerMap.get(slim.player_id);
							return (
								<button
									key={slim.player_id}
									type="button"
									className={`pop-item${i === activeIndex ? " active" : ""}`}
									onClick={() => handleSelect(slim)}
									onMouseEnter={() => setActiveIndex(i)}
								>
									<PlayerAvatar player={toPlayer(slim)} />
									<div style={{ minWidth: 0, flex: 1 }}>
										<div className="pname">
											{slim.first_name} {slim.last_name}
										</div>
										<div className="pmeta">{owner ? owner.teamName : "Free Agent"}</div>
									</div>
								</button>
							);
						})}
					</div>
				</div>
			)}
		</div>
	);
}
