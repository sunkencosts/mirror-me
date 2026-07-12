import { useCallback, useEffect, useRef, useState } from "react";

// Copies text to the clipboard and flashes a `copied` flag for ~1.5s. Falls back
// to a hidden textarea + execCommand in insecure contexts (no navigator.clipboard).
export function useCopyToClipboard(resetMs = 1500) {
	const [copied, setCopied] = useState(false);
	const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

	useEffect(() => {
		return () => {
			if (timeoutRef.current) {
				clearTimeout(timeoutRef.current);
			}
		};
	}, []);

	const copy = useCallback(
		async (text: string) => {
			try {
				if (navigator.clipboard) {
					await navigator.clipboard.writeText(text);
				} else {
					const input = document.createElement("textarea");
					input.value = text;
					input.style.position = "fixed";
					input.style.opacity = "0";
					document.body.append(input);
					input.select();
					document.execCommand("copy");
					input.remove();
				}
				setCopied(true);
				if (timeoutRef.current) {
					clearTimeout(timeoutRef.current);
				}
				timeoutRef.current = setTimeout(() => setCopied(false), resetMs);
			} catch {
				// clipboard unavailable in this context — nothing more we can do
			}
		},
		[resetMs],
	);

	return { copied, copy };
}
