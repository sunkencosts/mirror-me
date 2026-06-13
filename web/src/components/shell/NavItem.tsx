import { Link } from "react-router";
import { Icon, type IconName } from "../icons";

interface NavItemProps {
	to: string;
	icon: IconName;
	label: string;
	active: boolean;
	disabled?: boolean;
	onNavigate?: () => void;
}

export default function NavItem({ to, icon, label, active, disabled, onNavigate }: NavItemProps) {
	if (disabled) {
		return (
			<span className="nav-item disabled">
				<Icon name={icon} className="ic" />
				<span>{label}</span>
			</span>
		);
	}
	return (
		<Link to={to} className={`nav-item${active ? " active" : ""}`} onClick={onNavigate}>
			<Icon name={icon} className="ic" />
			<span>{label}</span>
		</Link>
	);
}
