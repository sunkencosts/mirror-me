import { RARITY_RING } from "../rarity";
import type { Player } from "../types";
import { onImageError } from "../utils/playerImage";

interface Props {
	player: Player;
	dimmed?: boolean;
}

/** Real Sleeper headshot wrapped in the design's `.pav` rarity ring. */
export default function PlayerAvatar({ player, dimmed }: Props) {
	const ring = dimmed ? "r-dim" : RARITY_RING[player.rarity ?? "grey"];
	return (
		<div className={`pav ${ring}`}>
			<img
				src={player.image_url}
				alt={`${player.first_name} ${player.last_name}`}
				onError={onImageError}
			/>
		</div>
	);
}
