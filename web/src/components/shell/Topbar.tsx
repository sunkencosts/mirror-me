import { useCurrentLeague } from "../../hooks/useCurrentLeague";
import { useLeagueName } from "../../hooks/useLeagueName";
import { routeByKey } from "./routes";

export default function Topbar() {
	const { routeKey } = useCurrentLeague();
	const leagueName = useLeagueName();
	const route = routeByKey(routeKey);
	const title = route?.title ?? "Mirror League";
	const crumb = route?.scope === "league" ? (leagueName ?? "League") : "Global";

	return (
		<div className="topbar">
			<div>
				<div className="crumb">
					<span>{crumb}</span>
				</div>
				<div className="page-title" style={{ marginTop: 5 }}>
					{title}
				</div>
			</div>

		</div>
	);
}
