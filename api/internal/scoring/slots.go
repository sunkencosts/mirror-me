// Package scoring holds the lineup-legality and (later) optimal-lineup/efficiency logic
// shared by the lineup-write path and the leaderboard grader. It is pure and
// dependency-light so it can be unit-tested in isolation.
package scoring

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
)

// SlotEligibility maps a starting slot to the fantasy positions allowed in it. It mirrors
// the frontend's SLOT_ELIGIBILITY (web/src/slots.ts) for the v1.0-supported slots. Slots
// NOT in this map (IDP, REC_FLEX, WRRB_FLEX, …) are treated as "exotic" and fail open
// (see ValidateLineup), per decision D16.
var SlotEligibility = map[string][]string{
	"QB":         {"QB"},
	"RB":         {"RB"},
	"WR":         {"WR"},
	"TE":         {"TE"},
	"K":          {"K"},
	"DEF":        {"DEF"},
	"FLEX":       {"RB", "WR", "TE"},
	"SUPER_FLEX": {"QB", "RB", "WR", "TE"},
}

// nonStartingSlots are roster_positions entries that are not part of the starting lineup.
var nonStartingSlots = map[string]struct{}{
	"BN": {}, "IR": {}, "TAXI": {},
}

// StartingSlots returns the starting slots from a league's roster_positions, dropping
// bench/reserve markers (mirrors the frontend's `roster_positions.filter(p => p !== "BN")`).
func StartingSlots(rosterPositions []string) []string {
	slots := make([]string, 0, len(rosterPositions))
	for _, p := range rosterPositions {
		if _, skip := nonStartingSlots[p]; skip {
			continue
		}
		slots = append(slots, p)
	}
	return slots
}

// ValidateLineup reports whether `starters` legally fills the league's starting slots:
// exact count, no duplicates, and a perfect assignment of players to slots respecting
// position eligibility (FLEX = RB/WR/TE, SUPER_FLEX = +QB). playerPositions maps each
// starter's id to its fantasy positions.
//
// Fail-open cases (return nil, validation skipped) per D16/D17:
//   - no starting slots known (e.g. roster_positions not published) — can't validate;
//   - any starting slot is exotic/unsupported — log a warning and skip.
//
// Note: membership ("player is on this roster") is checked by the caller before this.
func ValidateLineup(ctx context.Context, rosterPositions, starters []string, playerPositions map[string][]string) error {
	slots := StartingSlots(rosterPositions)
	if len(slots) == 0 {
		return nil // nothing to validate against — fail open
	}
	if bad, ok := firstUnsupportedSlot(slots); ok {
		slog.WarnContext(ctx, "unsupported roster slot; skipping positional validation (fail open)",
			slog.String("slot", bad))
		return nil
	}

	if len(starters) != len(slots) {
		return fmt.Errorf("expected %d starters, got %d", len(slots), len(starters))
	}

	seen := make(map[string]struct{}, len(starters))
	for _, id := range starters {
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate starter %s", id)
		}
		seen[id] = struct{}{}
	}

	if slot, ok := assignSlots(slots, starters, playerPositions); !ok {
		return fmt.Errorf("no legal assignment for slot %s", slot)
	}
	return nil
}

func firstUnsupportedSlot(slots []string) (string, bool) {
	for _, s := range slots {
		if _, ok := SlotEligibility[s]; !ok {
			return s, true
		}
	}
	return "", false
}

// eligible reports whether a player (by its positions) may fill a slot.
func eligible(playerPositions map[string][]string, player, slot string) bool {
	allowed := SlotEligibility[slot]
	for _, pos := range playerPositions[player] {
		if slices.Contains(allowed, pos) {
			return true
		}
	}
	return false
}

// assignSlots attempts a perfect matching of slots→players (Kuhn's augmenting-path
// algorithm). With len(slots) == len(starters), a full matching means every slot is
// filled by a distinct eligible player. On failure it returns the first slot that could
// not be matched. FLEX-correct by construction: it searches all assignments, not greedy.
func assignSlots(slots, starters []string, playerPositions map[string][]string) (string, bool) {
	playerToSlot := make([]int, len(starters))
	for i := range playerToSlot {
		playerToSlot[i] = -1
	}

	var augment func(slotIdx int, visited []bool) bool
	augment = func(slotIdx int, visited []bool) bool {
		for pi, player := range starters {
			if visited[pi] || !eligible(playerPositions, player, slots[slotIdx]) {
				continue
			}
			visited[pi] = true
			if playerToSlot[pi] == -1 || augment(playerToSlot[pi], visited) {
				playerToSlot[pi] = slotIdx
				return true
			}
		}
		return false
	}

	for si := range slots {
		visited := make([]bool, len(starters))
		if !augment(si, visited) {
			return slots[si], false
		}
	}
	return "", true
}
