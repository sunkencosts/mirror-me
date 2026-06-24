package scoring

import (
	"context"
	"testing"
)

var stdSlots = []string{"QB", "RB", "RB", "WR", "WR", "TE", "FLEX", "K", "DEF", "BN", "BN"}
var sfSlots = []string{"QB", "RB", "RB", "WR", "WR", "TE", "SUPER_FLEX", "K", "DEF", "BN"}

// pos is a tiny helper to declare a player's positions inline.
func positions(p map[string][]string) map[string][]string { return p }

func TestStartingSlots_DropsBench(t *testing.T) {
	got := StartingSlots(stdSlots)
	if len(got) != 9 {
		t.Fatalf("expected 9 starting slots, got %d (%v)", len(got), got)
	}
	for _, s := range got {
		if s == "BN" {
			t.Errorf("BN should have been dropped: %v", got)
		}
	}
}

func TestValidateLineup_LegalStandard(t *testing.T) {
	pp := positions(map[string][]string{
		"qb": {"QB"}, "rb1": {"RB"}, "rb2": {"RB"}, "wr1": {"WR"}, "wr2": {"WR"},
		"te": {"TE"}, "flexrb": {"RB"}, "k": {"K"}, "def": {"DEF"},
	})
	starters := []string{"qb", "rb1", "rb2", "wr1", "wr2", "te", "flexrb", "k", "def"}
	if err := ValidateLineup(context.Background(), stdSlots, starters, pp); err != nil {
		t.Fatalf("expected legal, got %v", err)
	}
}

func TestValidateLineup_WrongCount(t *testing.T) {
	pp := positions(map[string][]string{"qb": {"QB"}})
	if err := ValidateLineup(context.Background(), stdSlots, []string{"qb"}, pp); err == nil {
		t.Fatal("expected error for too few starters")
	}
}

func TestValidateLineup_DuplicateRejected(t *testing.T) {
	pp := positions(map[string][]string{
		"qb": {"QB"}, "rb1": {"RB"}, "wr1": {"WR"}, "wr2": {"WR"},
		"te": {"TE"}, "k": {"K"}, "def": {"DEF"},
	})
	// rb1 listed twice (9 entries) — duplicate must be caught.
	starters := []string{"qb", "rb1", "rb1", "wr1", "wr2", "te", "rb1", "k", "def"}
	if err := ValidateLineup(context.Background(), stdSlots, starters, pp); err == nil {
		t.Fatal("expected duplicate-starter error")
	}
}

func TestValidateLineup_BadPositionMix(t *testing.T) {
	// 4 RB, 1 WR — can't fill WR×2 (only one WR, FLEX can't make a second).
	pp := positions(map[string][]string{
		"qb": {"QB"}, "rb1": {"RB"}, "rb2": {"RB"}, "rb3": {"RB"}, "rb4": {"RB"},
		"wr1": {"WR"}, "te": {"TE"}, "k": {"K"}, "def": {"DEF"},
	})
	starters := []string{"qb", "rb1", "rb2", "rb3", "rb4", "wr1", "te", "k", "def"}
	if err := ValidateLineup(context.Background(), stdSlots, starters, pp); err == nil {
		t.Fatal("expected bad-position-mix error")
	}
}

func TestValidateLineup_FlexAbsorbsExtraRB(t *testing.T) {
	pp := positions(map[string][]string{
		"qb": {"QB"}, "rb1": {"RB"}, "rb2": {"RB"}, "rb3": {"RB"},
		"wr1": {"WR"}, "wr2": {"WR"}, "te": {"TE"}, "k": {"K"}, "def": {"DEF"},
	})
	starters := []string{"qb", "rb1", "rb2", "wr1", "wr2", "te", "rb3", "k", "def"}
	if err := ValidateLineup(context.Background(), stdSlots, starters, pp); err != nil {
		t.Fatalf("third RB should fill FLEX: %v", err)
	}
}

func TestValidateLineup_QBCannotFillStandardFlex(t *testing.T) {
	pp := positions(map[string][]string{
		"qb1": {"QB"}, "qb2": {"QB"}, "rb1": {"RB"}, "rb2": {"RB"},
		"wr1": {"WR"}, "wr2": {"WR"}, "te": {"TE"}, "k": {"K"}, "def": {"DEF"},
	})
	// Second QB has nowhere to go in a standard league (FLEX is RB/WR/TE only).
	starters := []string{"qb1", "qb2", "rb1", "rb2", "wr1", "wr2", "te", "k", "def"}
	if err := ValidateLineup(context.Background(), stdSlots, starters, pp); err == nil {
		t.Fatal("expected QB-in-FLEX rejection in standard league")
	}
}

func TestValidateLineup_SuperFlexAllowsSecondQB(t *testing.T) {
	pp := positions(map[string][]string{
		"qb1": {"QB"}, "qb2": {"QB"}, "rb1": {"RB"}, "rb2": {"RB"},
		"wr1": {"WR"}, "wr2": {"WR"}, "te": {"TE"}, "k": {"K"}, "def": {"DEF"},
	})
	starters := []string{"qb1", "qb2", "rb1", "rb2", "wr1", "wr2", "te", "k", "def"}
	if err := ValidateLineup(context.Background(), sfSlots, starters, pp); err != nil {
		t.Fatalf("SUPER_FLEX should allow a second QB: %v", err)
	}
}

// TestValidateLineup_CorrectUnderFlexNotNaiveGreedy is the key correctness case: a naive
// greedy that fills FLEX early with the only TE would strand the TE slot. A correct
// matching must route the dual-eligible player to FLEX and keep the TE for the TE slot.
func TestValidateLineup_CorrectUnderFlexNotNaiveGreedy(t *testing.T) {
	slots := []string{"TE", "FLEX"}
	pp := positions(map[string][]string{
		"te_only": {"TE"}, // can ONLY play TE
		"rb":      {"RB"}, // can only play FLEX here
	})
	// If FLEX greedily grabs te_only, the TE slot can't be filled by rb. A correct
	// algorithm assigns rb→FLEX and te_only→TE.
	starters := []string{"te_only", "rb"}
	if err := ValidateLineup(context.Background(), slots, starters, pp); err != nil {
		t.Fatalf("expected legal via augmenting reassignment, got %v", err)
	}
}

func TestValidateLineup_ExoticSlotFailsOpen(t *testing.T) {
	slots := []string{"QB", "IDP_FLEX", "DL"}
	// Wrong count and nonsense positions — but exotic slots make it fail open (nil).
	pp := positions(map[string][]string{"qb": {"QB"}})
	if err := ValidateLineup(context.Background(), slots, []string{"qb"}, pp); err != nil {
		t.Fatalf("exotic slots should fail open (nil), got %v", err)
	}
}

func TestValidateLineup_NoSlotsFailsOpen(t *testing.T) {
	if err := ValidateLineup(context.Background(), nil, []string{"a", "b"}, nil); err != nil {
		t.Fatalf("missing roster_positions should fail open, got %v", err)
	}
}
