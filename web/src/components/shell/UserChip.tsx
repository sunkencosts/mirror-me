import type { AuthUser } from "../../types";
import { avatarBg, initials } from "../../utils/avatar";
import { Icon } from "../icons";

interface UserChipProps {
	user: AuthUser | null;
	isLoading: boolean;
	onLogout: () => void;
}

export default function UserChip({ user, isLoading, onLogout }: UserChipProps) {
	if (isLoading) {
		return (
			<div className="user-chip" aria-hidden="true">
				<div className="av pav" style={{ width: 28, height: 28 }} />
			</div>
		);
	}
	if (!user) {
		return (
			<button
				type="button"
				className="user-chip"
				onClick={() => {
					window.location.href = `${import.meta.env.VITE_API_URL ?? ""}/auth/google`;
				}}
				style={{ textAlign: "left" }}
			>
				<div
					className="av pav r-unc"
					style={{ background: avatarBg("guest"), width: 28, height: 28, fontSize: 11 }}
				>
					?
				</div>
				<div style={{ flex: 1, minWidth: 0 }}>
					<div className="u-name">Sign in</div>
					<div className="u-sub">Save your lineups</div>
				</div>
				<Icon name="logout" className="out" />
			</button>
		);
	}
	return (
		<div className="user-chip">
			<div
				className="av pav r-myt"
				style={{ background: avatarBg(user.username), width: 28, height: 28, fontSize: 11 }}
			>
				{initials(user.username)}
			</div>
			<div style={{ flex: 1, minWidth: 0 }}>
				<div className="u-name">{user.username}</div>
				<div className="u-sub">{user.email}</div>
			</div>
			<button type="button" className="out" title="Log out" onClick={onLogout} aria-label="Log out">
				<Icon name="logout" />
			</button>
		</div>
	);
}
