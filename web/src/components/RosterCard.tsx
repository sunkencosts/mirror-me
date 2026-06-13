import { useEffect, useRef, useState } from "react";
import { useRosterCard } from "../hooks/useRosterCard";
import { slotLabel, slotPosClass } from "../slots";
import type { Lineup, Player, Roster, WeekMatchup } from "../types";
import { EyeIcon, EyeSlashIcon, Icon } from "./icons";
import PlayerCard from "./PlayerCard";
import PlayerPickerItem from "./PlayerPickerItem";
import RosterRarity from "./RosterRarity";

interface Props {
	roster: Roster;
	weekMatchup?: WeekMatchup | null;
	starterSlots: string[];
	irSlots: number;
	taxiSlots: number;
	allScores: number[];
	leagueId: string;
	weekNumber: number;
	currentWeek: number;
	userId: string;
	lineups: Lineup[];
	isDismissed: boolean;
	onToggleDismiss: () => void;
}

interface StarterRowProps {
	slot: string;
	officialPlayer: Player | null;
	overridePlayer: Player | null;
	isPickerOpen: boolean;
	eligiblePicks: Player[];
	onTogglePicker: () => void;
	onPickOverride: (player: Player) => void;
	onClearOverride: () => void;
	officialPoints?: number;
	overridePoints?: number;
	pointsFor: (playerId: string) => number | undefined;
}

function StarterRow({
	slot,
	officialPlayer,
	overridePlayer,
	isPickerOpen,
	eligiblePicks,
	onTogglePicker,
	onPickOverride,
	onClearOverride,
	officialPoints,
	overridePoints,
	pointsFor,
}: StarterRowProps) {
	const [query, setQuery] = useState("");
	const searchRef = useRef<HTMLInputElement>(null);
	const isOverridden = overridePlayer !== null;
	const posCls = slotPosClass(slot);
	const delta =
		officialPoints !== undefined && overridePoints !== undefined
			? overridePoints - officialPoints
			: null;

	useEffect(() => {
		if (isPickerOpen) {
			searchRef.current?.focus();
		} else {
			setQuery("");
		}
	}, [isPickerOpen]);

	const filtered = query
		? eligiblePicks.filter((p) =>
				`${p.first_name} ${p.last_name}`.toLowerCase().includes(query.toLowerCase()),
			)
		: eligiblePicks;

	return (
		<div className={`prow${isOverridden ? " scored" : ""}`}>
			{officialPlayer ? (
				<PlayerCard player={officialPlayer} points={officialPoints} dimmed={isOverridden} />
			) : (
				<div className="empty-slot">
					<div className="ring" />
					<span>Empty</span>
				</div>
			)}

			<div className="pos-cell">
				{isOverridden && delta !== null ? (
					<div className="stack">
						<span className={`pos ${posCls}`}>{slotLabel(slot)}</span>
						<span className={`delta ${delta >= 0 ? "pos" : "neg"} tnum`}>
							{delta >= 0 ? "+" : ""}
							{delta.toFixed(1)}
						</span>
					</div>
				) : (
					<span className={`pos ${posCls}`}>{slotLabel(slot)}</span>
				)}
			</div>

			<div className="your-cell">
				{isOverridden ? (
					<button
						type="button"
						className="your-pick-btn"
						onClick={onClearOverride}
						title="Click to remove override"
					>
						<PlayerCard player={overridePlayer} reversed points={overridePoints} />
					</button>
				) : (
					<>
						<button type="button" className="override-btn" onClick={onTogglePicker}>
							<Icon name="plus" />
							Override
						</button>
						{isPickerOpen && (
							<div className="pop">
								<div className="pop-search">
									<Icon name="search" />
									<input
										ref={searchRef}
										value={query}
										onChange={(e) => setQuery(e.target.value)}
										placeholder={`Search ${slotLabel(slot)}s…`}
									/>
								</div>
								<div className="pop-list">
									{filtered.length > 0 ? (
										filtered.map((p) => (
											<PlayerPickerItem
												key={p.player_id}
												player={p}
												points={pointsFor(p.player_id)}
												onClick={() => onPickOverride(p)}
											/>
										))
									) : (
										<div className="pop-empty">No eligible bench players</div>
									)}
								</div>
							</div>
						)}
					</>
				)}
			</div>
		</div>
	);
}

export default function RosterCard({
	roster,
	weekMatchup,
	starterSlots,
	irSlots,
	taxiSlots,
	allScores,
	leagueId,
	weekNumber,
	currentWeek,
	userId,
	lineups,
	isDismissed,
	onToggleDismiss,
}: Props) {
	const [benchOpen, setBenchOpen] = useState(false);

	const {
		activePlayers,
		activeStarters,
		activeReserve,
		activeTaxi,
		playerPoints,
		weekHasScoring,
		officialPoints,
		userTotal,
		diff,
		winner,
		hasOverrides,
		overrides,
		bench,
		eligiblePicksBySlot,
		slotKeys,
		selectedIndex,
		showAuthPrompt,
		handleTogglePicker,
		handlePickOverride,
		handleClearOverride,
		handleCloseAllPickers,
	} = useRosterCard({
		roster,
		weekMatchup,
		starterSlots,
		lineups,
		userId,
		leagueId,
		weekNumber,
		currentWeek,
	});

	function getPoints(playerId: string) {
		return weekHasScoring ? (playerPoints[playerId] ?? 0) : undefined;
	}

	const verdictClass = winner === "user" ? "win" : winner === "official" ? "lose" : "tie";
	const verdictText = winner ? { user: "You Win", official: "You Lose", tie: "Tie" }[winner] : "";
	const showVerdict = hasOverrides && winner !== null;
	const yourScoreColor =
		winner === "user" ? "var(--win)" : winner === "official" ? "var(--lose)" : undefined;

	return (
		<div className={`team${isDismissed ? " dismissed" : ""}`}>
			<div className="team-head">
				<button
					type="button"
					className="eye"
					aria-label={isDismissed ? "Restore team" : "Dismiss team to end"}
					onClick={onToggleDismiss}
				>
					{isDismissed ? <EyeSlashIcon /> : <EyeIcon />}
				</button>
				<h3>{roster.team_name || `Team ${roster.roster_id}`}</h3>
			</div>

			{!isDismissed && (
				<>
					<RosterRarity players={activePlayers} starters={activeStarters} allScores={allScores} />

					<div className="score-strip">
						<div className="col">
							<div className="l">Official Score</div>
							<div className="v tnum">
								{officialPoints != null ? officialPoints.toFixed(2) : "—"}
							</div>
						</div>
						{showVerdict && (
							<div className={`verdict ${verdictClass}`}>
								{verdictText}
								{diff !== null && (
									<span className="d tnum">
										{diff >= 0 ? "+" : ""}
										{diff.toFixed(2)}
									</span>
								)}
							</div>
						)}
						<div className="col right">
							<div className="l">Your Score</div>
							{hasOverrides ? (
								<div className="v tnum" style={{ color: yourScoreColor }}>
									{userTotal != null ? userTotal.toFixed(2) : "—"}
								</div>
							) : (
								<div className="v muted">No picks yet</div>
							)}
						</div>
					</div>

					<div className="roster-head">
						<span className="o">Official</span>
						<span className="y">Your Pick</span>
					</div>

					{showAuthPrompt && (
						<p className="auth-prompt">
							<a href={`${import.meta.env.VITE_API_URL ?? ""}/auth/google`}>Sign in</a> to save your
							lineup picks.
						</p>
					)}

					{starterSlots.map((slot, i) => {
						const official = activeStarters[i] ?? null;
						const overridePlayer = overrides[i] ?? null;
						return (
							<StarterRow
								key={slotKeys[i]}
								slot={slot}
								officialPlayer={official}
								overridePlayer={overridePlayer}
								isPickerOpen={selectedIndex === i}
								eligiblePicks={eligiblePicksBySlot[i]}
								onTogglePicker={() => handleTogglePicker(i)}
								onPickOverride={(player) => handlePickOverride(i, player)}
								onClearOverride={() => handleClearOverride(i)}
								officialPoints={official ? getPoints(official.player_id) : undefined}
								overridePoints={overridePlayer ? getPoints(overridePlayer.player_id) : undefined}
								pointsFor={getPoints}
							/>
						);
					})}

					<button
						type="button"
						className="bench"
						onClick={() => setBenchOpen((open) => !open)}
						aria-expanded={benchOpen}
					>
						Bench
						<Icon name="chevDown" />
					</button>
					{benchOpen && (
						<div className="bench-list">
							{bench.map((player) => (
								<div key={player.player_id} className="bench-row">
									<PlayerCard player={player} points={getPoints(player.player_id)} />
								</div>
							))}
							{irSlots > 0 && activeReserve.length > 0 && (
								<>
									<div className="bench-sublabel">IR</div>
									{activeReserve.map((player) => (
										<div key={player.player_id} className="bench-row">
											<PlayerCard player={player} points={getPoints(player.player_id)} />
										</div>
									))}
								</>
							)}
							{taxiSlots > 0 && activeTaxi.length > 0 && (
								<>
									<div className="bench-sublabel">Taxi</div>
									{activeTaxi.map((player) => (
										<div key={player.player_id} className="bench-row">
											<PlayerCard player={player} points={getPoints(player.player_id)} />
										</div>
									))}
								</>
							)}
						</div>
					)}

					{selectedIndex !== null && (
						<button
							type="button"
							aria-label="Close"
							className="pop-backdrop"
							onMouseDown={handleCloseAllPickers}
						/>
					)}
				</>
			)}
		</div>
	);
}
