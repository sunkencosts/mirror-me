import { memo } from "react";
import type { Player } from "../types";
import PlayerAvatar from "./PlayerAvatar";

interface Props {
	player: Player;
	reversed?: boolean;
	points?: number;
	dimmed?: boolean;
}

function PlayerCard({ player, reversed, points, dimmed }: Props) {
	const cls = ["player", reversed && "rev", dimmed && "dim"].filter(Boolean).join(" ");
	return (
		<div className={cls}>
			<PlayerAvatar player={player} dimmed={dimmed} />
			<div className="pinfo">
				<div className="pname">
					{player.first_name} {player.last_name}
				</div>
				<div className="pmeta">
					{player.fantasy_positions[0]} · {player.team}
					{typeof points === "number" && (
						<>
							{" · "}
							<span className="ppts tnum">{points.toFixed(1)} pts</span>
						</>
					)}
				</div>
			</div>
		</div>
	);
}

export default memo(PlayerCard);
