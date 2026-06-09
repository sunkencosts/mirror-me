import { useEffect, useRef, useState } from "react";

function storageKey(leagueId: string) {
	return `mirrorleague:dismissed:${leagueId}`;
}

function readFromStorage(leagueId: string): number[] {
	try {
		const stored = localStorage.getItem(storageKey(leagueId));
		return stored ? (JSON.parse(stored) as number[]) : [];
	} catch {
		return [];
	}
}

export function useDismissed(leagueId: string) {
	const [dismissedToEnd, setDismissedToEnd] = useState<number[]>(() =>
		readFromStorage(leagueId),
	);

	// Always reflects the current leagueId so the write effect below uses
	// the right storage key even when leagueId changes mid-render.
	const leagueIdRef = useRef(leagueId);
	leagueIdRef.current = leagueId;

	// Reset state when navigating to a different league.
	useEffect(() => {
		setDismissedToEnd(readFromStorage(leagueId));
	}, [leagueId]);

	const isMounted = useRef(false);

	// Persist changes. Skips the initial mount to avoid a redundant write of
	// the value we just read. Reads leagueId from the ref so this effect only
	// re-runs when the dismissed list changes, preventing stale key writes.
	useEffect(() => {
		if (!isMounted.current) {
			isMounted.current = true;
			return;
		}
		try {
			localStorage.setItem(storageKey(leagueIdRef.current), JSON.stringify(dismissedToEnd));
		} catch {
			// localStorage unavailable
		}
	}, [dismissedToEnd]);

	return [dismissedToEnd, setDismissedToEnd] as const;
}
