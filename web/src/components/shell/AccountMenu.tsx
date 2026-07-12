import { Link } from "react-router";
import type { AuthUser } from "../../types";
import { avatarBg, initials } from "../../utils/avatar";
import { Icon } from "../icons";

interface AccountMenuProps {
	user: AuthUser;
	onLogout: () => void;
	onClose: () => void;
	/** sidebar opens upward above the chip; appbar drops down under the mobile avatar. */
	variant: "sidebar" | "appbar";
}

// AccountMenu is the popover shown when the user taps their avatar (desktop chip or mobile app
// bar). It shows the identity header and links to the full account page; the actual username /
// display-name editing lives on /account.
export default function AccountMenu({ user, onLogout, onClose, variant }: AccountMenuProps) {
	return (
		<>
			<div className={`pop account-pop ${variant}`} role="menu">
				<div className="account-head">
					<div className="pav r-myt account-av" style={{ background: avatarBg(user.display_name) }}>
						{initials(user.display_name)}
					</div>
					<div style={{ minWidth: 0 }}>
						<div className="account-name">{user.display_name}</div>
						<div className="account-handle">@{user.username}</div>
					</div>
				</div>
				<Link to="/account" className="pop-item" role="menuitem" onClick={onClose}>
					<Icon name="users" />
					<div className="pname">Manage account</div>
				</Link>
				<button
					type="button"
					className="pop-item account-signout"
					role="menuitem"
					onClick={onLogout}
				>
					<Icon name="logout" />
					<div className="pname">Sign out</div>
				</button>
			</div>
			<button type="button" aria-label="Close" className="pop-backdrop" onClick={onClose} />
		</>
	);
}
