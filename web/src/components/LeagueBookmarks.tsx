import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { bookmarksKey, deleteJson, fetchJson, patchJson } from "../api";
import type { LeagueBookmark } from "../types";
import LeagueBookmarkCard from "./LeagueBookmarkCard";

interface Props {
	userId: string;
}

export default function LeagueBookmarks({ userId }: Props) {
	const queryClient = useQueryClient();
	const { data: bookmarks = [] } = useQuery<LeagueBookmark[]>({
		queryKey: bookmarksKey(userId),
		queryFn: () => fetchJson(`/league-bookmarks?user_id=${userId}`),
	});

	const [editingId, setEditingId] = useState<string | null>(null);
	const [editLabel, setEditLabel] = useState("");
	const editInputRef = useRef<HTMLInputElement>(null);

	useEffect(() => {
		if (editingId) {
			editInputRef.current?.focus();
		}
	}, [editingId]);

	const deleteMutation = useMutation({
		mutationFn: ({ leagueId, source }: { leagueId: string; source: string }) =>
			deleteJson(`/league-bookmarks/${leagueId}?user_id=${userId}&source=${source}`),
		onSuccess: () => queryClient.invalidateQueries({ queryKey: bookmarksKey(userId) }),
	});

	const patchMutation = useMutation({
		mutationFn: ({
			leagueId,
			label,
			source,
		}: {
			leagueId: string;
			label: string;
			source: string;
		}) =>
			patchJson<LeagueBookmark>(`/league-bookmarks/${leagueId}?source=${source}`, {
				user_id: userId,
				label,
			}),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: bookmarksKey(userId) });
			setEditingId(null);
		},
	});

	function startEdit(b: LeagueBookmark) {
		setEditingId(b.league_id);
		setEditLabel(b.label);
	}

	function cancelEdit() {
		setEditingId(null);
	}

	function saveEdit(leagueId: string) {
		const source = bookmarks.find((b) => b.league_id === leagueId)?.source ?? "";
		patchMutation.mutate({ leagueId, label: editLabel.trim(), source });
	}

	function handleEditKeyDown(e: React.KeyboardEvent, leagueId: string) {
		if (e.key === "Enter") {
			saveEdit(leagueId);
		}
		if (e.key === "Escape") {
			cancelEdit();
		}
	}

	return (
		<>
			<div className="section-label" style={{ display: "block", marginBottom: 14 }}>
				Your leagues
			</div>
			{bookmarks.length === 0 ? (
				<div className="panel">
					<div className="sub" style={{ margin: 0 }}>
						No leagues yet — connect one above to start mirroring.
					</div>
				</div>
			) : (
				<div className="lg-cards">
					{bookmarks.map((b) => {
						const isEditing = editingId === b.league_id;
						const isDeleting =
							deleteMutation.isPending && deleteMutation.variables?.leagueId === b.league_id;

						return (
							<LeagueBookmarkCard
								key={b.league_id}
								bookmark={b}
								isEditing={isEditing}
								editLabel={editLabel}
								editInputRef={editInputRef}
								onEditLabelChange={setEditLabel}
								onStartEdit={() => startEdit(b)}
								onCancelEdit={cancelEdit}
								onSaveEdit={() => saveEdit(b.league_id)}
								onKeyDown={(e) => handleEditKeyDown(e, b.league_id)}
								onDelete={() => deleteMutation.mutate({ leagueId: b.league_id, source: b.source })}
								isSaving={patchMutation.isPending}
								isDeleting={isDeleting}
							/>
						);
					})}
				</div>
			)}
		</>
	);
}
