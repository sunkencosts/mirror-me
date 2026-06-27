import { useState } from "react";
import type { Roster } from "../types";
import { Icon, type IconName } from "./icons";

interface Props {
	rosters: Roster[];
	/** Selected roster id. When set, the trigger shows that team's name. Leave unset for a
	    "jump to" picker that always shows the placeholder. */
	value?: number | null;
	onSelect: (rosterId: number) => void;
	/** Trigger text shown when nothing is selected. */
	placeholder?: string;
	/** Leading icon in the trigger. */
	icon?: IconName;
	/** Accessible label for the trigger button. */
	ariaLabel?: string;
}

function teamName(roster: Roster): string {
	return roster.team_name || `Team ${roster.roster_id}`;
}

/** Reusable team dropdown: a styled trigger plus a popover list of teams. Shared by the
    Lineups "jump to team" picker and the Results team selector. */
export default function TeamSelect({
	rosters,
	value,
	onSelect,
	placeholder = "Select team…",
	icon = "users",
	ariaLabel = "Select team",
}: Props) {
	const [open, setOpen] = useState(false);
	const selected = value != null ? rosters.find((roster) => roster.roster_id === value) : undefined;
	const sortedRosters = [...rosters].sort((a, b) =>
		teamName(a).localeCompare(teamName(b), undefined, { sensitivity: "base" }),
	);

	return (
		<div className="select">
			<button
				type="button"
				className="select-trigger"
				aria-label={ariaLabel}
				aria-expanded={open}
				onClick={() => setOpen((isOpen) => !isOpen)}
			>
				<Icon name={icon} />
				<span>{selected ? teamName(selected) : placeholder}</span>
				<Icon name="chevDown" />
			</button>
			{open && (
				<>
					<button
						type="button"
						aria-label="Close"
						className="pop-backdrop"
						onMouseDown={() => setOpen(false)}
					/>
					<div className="pop team-pop">
						<div className="pop-list">
							{sortedRosters.map((roster) => (
								<button
									key={roster.roster_id}
									type="button"
									className={`pop-item${roster.roster_id === value ? " active" : ""}`}
									onClick={() => {
										onSelect(roster.roster_id);
										setOpen(false);
									}}
								>
									<span className="pname">{teamName(roster)}</span>
								</button>
							))}
						</div>
					</div>
				</>
			)}
		</div>
	);
}
