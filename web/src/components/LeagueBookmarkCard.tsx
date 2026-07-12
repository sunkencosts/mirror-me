import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { fetchJson } from "../api";
import { useCopyToClipboard } from "../hooks/useCopyToClipboard";
import type { League, LeagueBookmark } from "../types";
import { pprLabel } from "../utils/league";
import { onImageError } from "../utils/playerImage";
import { CheckIcon, ShareIcon, TrashIcon, XIcon } from "./icons";

interface Props {
	bookmark: LeagueBookmark;
	isEditing: boolean;
	editLabel: string;
	editInputRef: React.RefObject<HTMLInputElement | null>;
	onEditLabelChange: (value: string) => void;
	onStartEdit: () => void;
	onCancelEdit: () => void;
	onSaveEdit: () => void;
	onKeyDown: (e: React.KeyboardEvent) => void;
	onDelete: () => void;
	isSaving: boolean;
	isDeleting: boolean;
}

export default function LeagueBookmarkCard({
	bookmark,
	isEditing,
	editLabel,
	editInputRef,
	onEditLabelChange,
	onCancelEdit,
	onSaveEdit,
	onKeyDown,
	onDelete,
	isSaving,
	isDeleting,
}: Props) {
	const navigate = useNavigate();
	const { copied, copy } = useCopyToClipboard();
	const { data: league } = useQuery<League>({
		queryKey: ["league", bookmark.league_id],
		queryFn: () => fetchJson(`/league/${bookmark.league_id}`),
	});

	const name = bookmark.label || bookmark.league_id;
	const ppr = league ? pprLabel(league.scoring_settings.rec) : null;
	const tep = (league?.scoring_settings.bonus_rec_te ?? 0) > 0;
	const superflex = league?.roster_positions.includes("SUPER_FLEX") ?? false;

	return (
		<div
			className="lg-card"
			style={{ cursor: isEditing ? "default" : "pointer" }}
			onClick={() => {
				if (!isEditing) navigate(`/${bookmark.league_id}/lineups`);
			}}
		>
			<div className="top">
				<div className="lg-av">
					{bookmark.icon_url ? (
						<img src={bookmark.icon_url} alt="" onError={onImageError} />
					) : (
						"⛳"
					)}
				</div>
				<div style={{ flex: 1, minWidth: 0 }}>
					{isEditing ? (
						<input
							ref={editInputRef}
							className="field"
							style={{ height: 32 }}
							value={editLabel}
							onChange={(e) => onEditLabelChange(e.target.value)}
							onKeyDown={onKeyDown}
							onClick={(e) => e.stopPropagation()}
						/>
					) : (
						<>
							<div className="nm">{name}</div>
							<div className="id">{bookmark.league_id}</div>
						</>
					)}
				</div>
				<div style={{ display: "flex", gap: 6 }}>
					{isEditing ? (
						<>
							<button
								type="button"
								className="icon-btn"
								style={{ width: 30, height: 30 }}
								onClick={(e) => {
									e.stopPropagation();
									onSaveEdit();
								}}
								disabled={isSaving}
								aria-label="Save"
								title="Save"
							>
								<CheckIcon />
							</button>
							<button
								type="button"
								className="icon-btn"
								style={{ width: 30, height: 30 }}
								onClick={(e) => {
									e.stopPropagation();
									onCancelEdit();
								}}
								aria-label="Cancel"
								title="Cancel"
							>
								<XIcon />
							</button>
						</>
					) : (
						<>
							<span className="copied-anchor">
								<button
									type="button"
									className="icon-btn"
									style={{ width: 30, height: 30 }}
									onClick={(e) => {
										e.stopPropagation();
										copy(`${window.location.origin}/add/${bookmark.league_id}`);
									}}
									aria-label={copied ? "Link copied!" : "Copy share link"}
									title={copied ? "Link copied!" : "Copy share link"}
								>
									{copied ? <CheckIcon /> : <ShareIcon />}
								</button>
								{/* Persistent live region (toggled via opacity, not mount/unmount)
								    so screen readers announce the text change on copy. */}
								<span className={`copied-pill${copied ? " show" : ""}`} role="status">
									{copied ? "Copied!" : ""}
								</span>
							</span>
							<button
								type="button"
								className="icon-btn"
								style={{ width: 30, height: 30 }}
								onClick={(e) => {
									e.stopPropagation();
									onDelete();
								}}
								disabled={isDeleting}
								aria-label="Delete bookmark"
								title="Delete bookmark"
							>
								<TrashIcon />
							</button>
						</>
					)}
				</div>
			</div>
			<div className="lg-card-footer">
				<div className="meta">
					{league && <span className="tag">{league.settings.num_teams} Teams</span>}
					{ppr && <span className="tag green">{ppr}</span>}
					{tep && <span className="tag green">TEP +{league?.scoring_settings.bonus_rec_te}</span>}
					{superflex && <span className="tag purple">Superflex</span>}
				</div>
			</div>
		</div>
	);
}
