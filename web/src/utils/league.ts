export function pprLabel(rec: number): string | null {
	if (rec === 1) {
		return "Full PPR";
	}
	if (rec === 0.5) {
		return "Half PPR";
	}
	return null;
}
