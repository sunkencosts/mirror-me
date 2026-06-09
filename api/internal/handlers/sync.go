package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/sunkencosts/mirrorleague/internal/provider"
	"github.com/sunkencosts/mirrorleague/internal/sleeper"
)

type playerSyncer interface {
	UpsertPlayers(ctx context.Context, players []provider.Player) error
}

type rosterInvalidator interface {
	InvalidateRosters()
}

func HandleSyncPlayers(store playerSyncer, invalidator rosterInvalidator, sleeperBaseURL, rankingsCSVURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			wg          sync.WaitGroup
			playerMap   map[string]provider.Player
			rarities    map[string]string
			playerErr   error
			rankingsErr error
		)
		ctx := r.Context()
		wg.Add(2)
		go func() { defer wg.Done(); playerMap, playerErr = sleeper.FetchPlayers(ctx, sleeperBaseURL) }()
		go func() { defer wg.Done(); rarities, rankingsErr = fetchRarities(ctx, rankingsCSVURL) }()
		wg.Wait()

		if playerErr != nil {
			http.Error(w, fmt.Sprintf("failed to fetch players: %v", playerErr), http.StatusServiceUnavailable)
			return
		}
		if rankingsErr != nil {
			http.Error(w, fmt.Sprintf("failed to fetch rankings: %v", rankingsErr), http.StatusServiceUnavailable)
			return
		}

		players := make([]provider.Player, 0, len(playerMap))
		for _, p := range playerMap {
			if len(p.FantasyPositions) > 0 {
				if rarity, ok := rarities[rarityKey(p.FirstName+" "+p.LastName, p.FantasyPositions[0])]; ok {
					p.Rarity = rarity
				}
			}
			players = append(players, p)
		}

		if err := store.UpsertPlayers(r.Context(), players); err != nil {
			http.Error(w, "failed to upsert players", http.StatusInternalServerError)
			return
		}
		invalidator.InvalidateRosters()

		_ = encode(w, r, http.StatusOK, map[string]int{"upserted": len(players)})
	})
}

const (
	csvColPageType  = 1
	csvColPos       = 5
	csvColMergeName = 22
)

var dynastyPositions = map[string]bool{
	"dynasty-qb": true, "dynasty-rb": true,
	"dynasty-wr": true, "dynasty-te": true,
}

func fetchRarities(ctx context.Context, csvURL string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, csvURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating rankings request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching rankings CSV: %w", err)
	}
	defer resp.Body.Close()

	byPos := map[string][]string{}
	r := csv.NewReader(resp.Body)
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("reading rankings CSV header: %w", err)
	}
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing rankings CSV: %w", err)
		}
		if !dynastyPositions[row[csvColPageType]] {
			continue
		}
		byPos[row[csvColPos]] = append(byPos[row[csvColPos]], row[csvColMergeName])
	}

	rarities := make(map[string]string)
	for pos, names := range byPos {
		total := len(names)
		for rank, name := range names {
			pct := float64(rank+1) / float64(total)
			rarity := rarityFromPct(pct)
			if rank == 0 {
				rarity = "mythic"
			}
			rarities[rarityKey(name, pos)] = rarity
		}
	}
	return rarities, nil
}

func rarityKey(name, pos string) string {
	return normalizeName(name) + "|" + pos
}

func normalizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, "’", "")
	for _, suffix := range []string{" iv", " iii", " ii", " jr", " sr"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return strings.TrimSpace(name)
}

func rarityFromPct(pct float64) string {
	switch {
	case pct <= 0.02:
		return "mythic"
	case pct <= 0.08:
		return "orange"
	case pct <= 0.20:
		return "purple"
	case pct <= 0.45:
		return "blue"
	case pct <= 0.75:
		return "green"
	default:
		return "grey"
	}
}
