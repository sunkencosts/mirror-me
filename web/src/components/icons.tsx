// Generic icon set ported from design_handoff_mirror_league/core.js (UI.icon).
// Rendered without a fixed width/height so size is controlled by CSS (e.g.
// `.nav-item .ic { width: 18px }`), matching the prototype's `icon(name, cls)`.
const ICON_PATHS: Record<string, string> = {
	trophy: "M8 21h8M12 17v4M7 4h10v5a5 5 0 0 1-10 0V4ZM17 5h3v2a3 3 0 0 1-3 3M7 5H4v2a3 3 0 0 0 3 3",
	list: "M8 6h12M8 12h12M8 18h12M3 6h.01M3 12h.01M3 18h.01",
	menu: "M3 6h18M3 12h18M3 18h18",
	ellipsis: "",
	stack: "m12 2 9 5-9 5-9-5 9-5ZM3 12l9 5 9-5M3 17l9 5 9-5",
	grid: "",
	chart: "",
	medal: "",
	users: "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M22 21v-2a4 4 0 0 0-3-3.87",
	chevDown: "m6 9 6 6 6-6",
	chevR: "m9 6 6 6-6 6",
	chevL: "m15 6-6 6 6 6",
	search: "m21 21-4.3-4.3",
	logout: "M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9",
	plus: "M12 5v14M5 12h14",
	check: "M20 6 9 17l-5-5",
	info: "M12 16v-4M12 8h.01",
	share: "M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8M16 6l-4-4-4 4M12 2v13",
	arrowUp: "m18 15-6-6-6 6",
	arrowDown: "m6 9 6 6 6-6",
	edit: "M12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5Z",
	bolt: "M13 2 3 14h7l-1 8 10-12h-7l1-8Z",
	lock: "",
};

// Icons whose markup includes shapes (circles/rects) beyond a single path.
const ICON_EXTRAS: Record<string, React.ReactNode> = {
	ellipsis: (
		<>
			<circle cx="5" cy="12" r="1.4" />
			<circle cx="12" cy="12" r="1.4" />
			<circle cx="19" cy="12" r="1.4" />
		</>
	),
	grid: (
		<>
			<rect x="3" y="3" width="7" height="7" rx="1.5" />
			<rect x="14" y="3" width="7" height="7" rx="1.5" />
			<rect x="3" y="14" width="7" height="7" rx="1.5" />
			<rect x="14" y="14" width="7" height="7" rx="1.5" />
		</>
	),
	chart: (
		<>
			<path d="M3 3v18h18" />
			<rect x="7" y="11" width="3" height="6" rx="1" />
			<rect x="12" y="7" width="3" height="10" rx="1" />
			<rect x="17" y="13" width="3" height="4" rx="1" />
		</>
	),
	medal: (
		<>
			<circle cx="12" cy="14" r="6" />
			<path d="M12 14v0M9 4l1.5 5M15 4l-1.5 5" />
		</>
	),
	search: <circle cx="11" cy="11" r="7" />,
	info: <circle cx="12" cy="12" r="9" />,
	users: (
		<>
			<circle cx="9" cy="7" r="4" />
			<path d="M16 3.13a4 4 0 0 1 0 7.75" />
		</>
	),
	lock: (
		<>
			<rect x="4" y="11" width="16" height="10" rx="2" />
			<path d="M8 11V7a4 4 0 0 1 8 0v4" />
		</>
	),
};

export type IconName = keyof typeof ICON_PATHS;

interface IconProps {
	name: IconName;
	className?: string;
}

export function Icon({ name, className }: IconProps) {
	const path = ICON_PATHS[name];
	return (
		<svg
			className={className}
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			{path ? <path d={path} /> : null}
			{ICON_EXTRAS[name] ?? null}
		</svg>
	);
}

export function CheckIcon() {
	return (
		<svg
			width="15"
			height="15"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2.5"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			<polyline points="20,6 9,17 4,12" />
		</svg>
	);
}

export function XIcon() {
	return (
		<svg
			width="15"
			height="15"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			<line x1="18" y1="6" x2="6" y2="18" />
			<line x1="6" y1="6" x2="18" y2="18" />
		</svg>
	);
}

export function PencilIcon() {
	return (
		<svg
			width="15"
			height="15"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			<path d="M12 20h9" />
			<path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4Z" />
		</svg>
	);
}

export function EyeIcon() {
	return (
		<svg
			width="16"
			height="16"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
			<circle cx="12" cy="12" r="3" />
		</svg>
	);
}

export function EyeSlashIcon() {
	return (
		<svg
			width="16"
			height="16"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			<path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" />
			<line x1="1" y1="1" x2="23" y2="23" />
		</svg>
	);
}

export function TrashIcon() {
	return (
		<svg
			width="15"
			height="15"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
		>
			<polyline points="3,6 5,6 21,6" />
			<path d="M19 6l-.867 12.142A2 2 0 0 1 16.138 20H7.862a2 2 0 0 1-1.995-1.858L5 6" />
			<path d="M10 11v6M14 11v6" />
			<path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" />
		</svg>
	);
}
