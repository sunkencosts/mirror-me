import type { ReactNode } from "react";

interface ComingSoonProps {
	title: string;
	children?: ReactNode;
}

/** Placeholder panel for pages whose data layer isn't built yet. */
export default function ComingSoon({ title, children }: ComingSoonProps) {
	return (
		<div className="fade-in">
			{children}
			<div className="panel" style={{ textAlign: "center", padding: "56px 20px" }}>
				<h4>{title}</h4>
				<p className="sub" style={{ marginBottom: 0 }}>
					Coming soon — this page is part of the redesign and gets wired up in a later step.
				</p>
			</div>
		</div>
	);
}
