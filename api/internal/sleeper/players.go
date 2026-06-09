package sleeper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

func FetchPlayers(ctx context.Context, baseURL string) (map[string]provider.Player, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/players/nfl", nil)
	if err != nil {
		return nil, fmt.Errorf("creating players request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching players: %w", err)
	}
	defer resp.Body.Close()

	var players map[string]provider.Player
	if err := json.NewDecoder(resp.Body).Decode(&players); err != nil {
		return nil, fmt.Errorf("decoding players: %w", err)
	}
	return players, nil
}
