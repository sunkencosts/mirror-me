import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { ApiError, deleteJson, patchJson } from "../api";
import { Icon } from "../components/icons";
import { useAuth } from "../context/AuthContext";
import type { AuthUser } from "../types";
import { avatarBg, initials } from "../utils/avatar";

const USERNAME_PATTERN = /^[a-z0-9_]{3,20}$/;
const MAX_DISPLAY_NAME = 30;

export default function AccountPage() {
	const { user, isLoading } = useAuth();
	const queryClient = useQueryClient();
	const navigate = useNavigate();

	const [displayName, setDisplayName] = useState("");
	const [username, setUsername] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [saved, setSaved] = useState(false);

	// Prefill the fields once the current user resolves.
	useEffect(() => {
		if (user) {
			setDisplayName(user.display_name);
			setUsername(user.username);
		}
	}, [user]);

	const save = useMutation({
		mutationFn: (payload: { username: string; display_name: string }) =>
			patchJson<AuthUser>("/auth/profile", payload),
		onSuccess: (updated) => {
			queryClient.setQueryData(["auth"], updated);
			queryClient.invalidateQueries({ queryKey: ["auth"] });
			setError(null);
			setSaved(true);
		},
		onError: (err) => {
			setSaved(false);
			if (err instanceof ApiError && err.status === 409) {
				setError("That username is taken. Try another.");
			} else if (err instanceof ApiError && err.status === 400) {
				setError("Check your username and display name and try again.");
			} else {
				setError("Something went wrong. Try again.");
			}
		},
	});

	async function handleSignOut() {
		await deleteJson("/auth/logout");
		queryClient.invalidateQueries({ queryKey: ["auth"] });
		navigate("/");
	}

	if (isLoading) {
		return (
			<div className="fade-in account-page">
				<h1 className="page-title">Account</h1>
				<p className="mini" style={{ marginTop: 14 }}>
					Loading…
				</p>
			</div>
		);
	}

	if (!user) {
		return (
			<div className="fade-in account-page">
				<h1 className="page-title">Account</h1>
				<div className="card account-card">
					<p className="mini">Sign in to manage your account.</p>
					<button
						type="button"
						className="btn btn-primary"
						style={{ marginTop: 14 }}
						onClick={() => {
							window.location.href = `${import.meta.env.VITE_API_URL ?? ""}/auth/google`;
						}}
					>
						Sign in with Google
					</button>
				</div>
			</div>
		);
	}

	const normalizedUsername = username.trim().toLowerCase();
	const trimmedDisplayName = displayName.trim();
	const usernameValid = USERNAME_PATTERN.test(normalizedUsername);
	const displayNameValid =
		trimmedDisplayName.length >= 1 && trimmedDisplayName.length <= MAX_DISPLAY_NAME;
	const changed = normalizedUsername !== user.username || trimmedDisplayName !== user.display_name;
	const canSave = usernameValid && displayNameValid && changed && !save.isPending;

	function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
		event.preventDefault();
		if (!canSave) {
			return;
		}
		save.mutate({ username: normalizedUsername, display_name: trimmedDisplayName });
	}

	return (
		<div className="fade-in account-page">
			<h1 className="page-title">Account</h1>

			<form className="card account-card" onSubmit={handleSubmit}>
				<div className="account-identity">
					<div
						className="pav r-myt account-av-lg"
						style={{ background: avatarBg(trimmedDisplayName || user.display_name) }}
					>
						{initials(trimmedDisplayName || user.display_name)}
					</div>
					<div style={{ minWidth: 0 }}>
						<div className="account-name-lg">{trimmedDisplayName || user.display_name}</div>
						<div className="account-email">{user.email}</div>
					</div>
				</div>

				<label className="account-field" htmlFor="account-display-name">
					<span className="account-field-label">Display name</span>
					<input
						id="account-display-name"
						className="field"
						type="text"
						value={displayName}
						maxLength={MAX_DISPLAY_NAME}
						autoComplete="off"
						onChange={(event) => {
							setDisplayName(event.target.value);
							setError(null);
							setSaved(false);
						}}
					/>
					<span className="account-field-hint">
						Shown around the app. 1–{MAX_DISPLAY_NAME} characters.
					</span>
				</label>

				<label className="account-field" htmlFor="account-username">
					<span className="account-field-label">Username</span>
					<div className="account-username-input">
						<span className="account-at">@</span>
						<input
							id="account-username"
							type="text"
							value={username}
							autoComplete="off"
							autoCapitalize="none"
							spellCheck={false}
							onChange={(event) => {
								setUsername(event.target.value);
								setError(null);
								setSaved(false);
							}}
						/>
					</div>
					<span className="account-field-hint">
						Your unique handle. Lowercase letters, numbers, and underscores; 3–20 characters.
					</span>
				</label>

				{error && (
					<p className="mini account-msg error" role="alert">
						{error}
					</p>
				)}
				{saved && !error && (
					<p className="mini account-msg ok">
						<Icon name="check" /> Saved.
					</p>
				)}

				<div className="account-actions">
					<button type="submit" className="btn btn-primary" disabled={!canSave}>
						{save.isPending ? "Saving…" : "Save changes"}
					</button>
					<button type="button" className="btn btn-ghost" onClick={handleSignOut}>
						<Icon name="logout" />
						Sign out
					</button>
				</div>
			</form>
		</div>
	);
}
