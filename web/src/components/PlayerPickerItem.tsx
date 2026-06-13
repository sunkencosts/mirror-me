import type { Player } from "../types";
import { Icon } from "./icons";
import PlayerAvatar from "./PlayerAvatar";

interface Props {
	player: Player;
	points?: number;
	onClick: () => void;
}

export default function PlayerPickerItem({ player, points, onClick }: Props) {
	return (
		<button type="button" className="pop-item" onClick={onClick}>
			<PlayerAvatar player={player} />
			<div style={{ minWidth: 0 }}>
				<div className="pname">
					{player.first_name} {player.last_name}
				</div>
				<div className="pmeta">
					{player.fantasy_positions[0]} · {player.team}
					{typeof points === "number" && ` · ${points.toFixed(1)} pts`}
				</div>
			</div>
			<Icon name="check" className="pick" />
		</button>
	);
}
