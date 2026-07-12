import { useState } from "react";
import type { AuthUser } from "../../types";
import { avatarBg, initials } from "../../utils/avatar";
import { Icon } from "../icons";
import AccountMenu from "./AccountMenu";

interface UserChipProps {
	user: AuthUser | null;
	isLoading: boolean;
	onLogout: () => void;
}

export default function UserChip({ user, isLoading, onLogout }: UserChipProps) {
	const [open, setOpen] = useState(false);

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
		<div className="account-anchor account-anchor--chip">
			<button
				type="button"
				className="user-chip user-chip-btn"
				onClick={() => setOpen((o) => !o)}
				aria-expanded={open}
				aria-haspopup="menu"
			>
				<div
					className="av pav r-myt"
					style={{ background: avatarBg(user.display_name), width: 28, height: 28, fontSize: 11 }}
				>
					{initials(user.display_name)}
				</div>
				<div style={{ flex: 1, minWidth: 0 }}>
					<div className="u-name">{user.display_name}</div>
					<div className="u-sub">@{user.username}</div>
				</div>
				<Icon name="chevDown" className="chev" />
			</button>
			{open && (
				<AccountMenu
					user={user}
					onLogout={onLogout}
					onClose={() => setOpen(false)}
					variant="sidebar"
				/>
			)}
		</div>
	);
}
