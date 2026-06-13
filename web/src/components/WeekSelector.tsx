import { Icon } from "./icons";

interface Props {
	weekNumber: number;
	onWeekChange: (week: number) => void;
	min?: number;
	max?: number;
}

export default function WeekSelector({ weekNumber, onWeekChange, min = 1, max = 18 }: Props) {
	return (
		<div className="week-pick">
			<button
				type="button"
				disabled={weekNumber <= min}
				onClick={() => onWeekChange(Math.max(min, weekNumber - 1))}
				aria-label="Previous week"
			>
				<Icon name="chevL" />
			</button>
			<span className="w">Week {weekNumber}</span>
			<button
				type="button"
				disabled={weekNumber >= max}
				onClick={() => onWeekChange(Math.min(max, weekNumber + 1))}
				aria-label="Next week"
			>
				<Icon name="chevR" />
			</button>
		</div>
	);
}
