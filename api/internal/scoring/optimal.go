package scoring

import "math"

// OptimalLineup returns the highest-scoring LEGAL lineup obtainable from a roster for a
// week, and its point total. It is the denominator for lineup efficiency (D23).
//
// This is a max-weight assignment: fill every starting slot with a distinct, position-
// eligible player so the summed points are maximal. A naive greedy is INCORRECT (a broad
// SUPER_FLEX slot can grab a player a narrow FLEX then can't replace), so we solve it as a
// min-cost max-flow (costs are negated points), which finds the true optimum.
//
// Returns (nil, 0) when there are no known starting slots or any slot is exotic/unsupported
// (D16) — the caller treats optimal_total == 0 as "exclude this week" (H5).
func OptimalLineup(rosterPositions, players []string, points map[string]float64, playerPositions map[string][]string) ([]string, float64) {
	slots := StartingSlots(rosterPositions)
	if len(slots) == 0 {
		return nil, 0
	}
	if _, unsupported := firstUnsupportedSlot(slots); unsupported {
		return nil, 0
	}

	numSlots := len(slots)
	numPlayers := len(players)
	source := 0
	slotBase := 1
	playerBase := 1 + numSlots
	sink := 1 + numSlots + numPlayers

	g := newMinCostFlow(sink + 1)
	for i := range slots {
		g.addEdge(source, slotBase+i, 1, 0)
	}
	for j := range players {
		g.addEdge(playerBase+j, sink, 1, 0)
	}
	for i, slot := range slots {
		for j, player := range players {
			if eligible(playerPositions, player, slot) {
				g.addEdge(slotBase+i, playerBase+j, 1, -points[player])
			}
		}
	}

	flow, cost := g.minCostMaxFlow(source, sink)
	// If we can't fill every starting slot with a distinct eligible player, there is no
	// legal full lineup, so "best possible" is undefined. Exclude the week (optimal == 0,
	// H5) rather than return an understated denominator that would inflate efficiency.
	if flow < numSlots {
		return nil, 0
	}
	total := -cost

	lineup := make([]string, 0, numSlots)
	for i := range slots {
		for _, ei := range g.adj[slotBase+i] {
			e := g.edges[ei]
			if e.to >= playerBase && e.to < sink && e.flow > 0 {
				lineup = append(lineup, players[e.to-playerBase])
			}
		}
	}
	return lineup, total
}

// --- min-cost max-flow (successive shortest paths, SPFA for negative edge costs) -------

type flowEdge struct {
	to, cap, flow int
	cost          float64
}

type minCostFlow struct {
	edges []flowEdge
	adj   [][]int
	n     int
}

func newMinCostFlow(n int) *minCostFlow {
	return &minCostFlow{adj: make([][]int, n), n: n}
}

// addEdge adds a directed edge plus its zero-capacity residual reverse edge. The pair is
// stored adjacently so the reverse of edge i is i^1.
func (m *minCostFlow) addEdge(from, to, capacity int, cost float64) {
	m.adj[from] = append(m.adj[from], len(m.edges))
	m.edges = append(m.edges, flowEdge{to: to, cap: capacity, cost: cost})
	m.adj[to] = append(m.adj[to], len(m.edges))
	m.edges = append(m.edges, flowEdge{to: from, cap: 0, cost: -cost})
}

// minCostMaxFlow pushes maximum flow from s to t at minimum cost. All capacities here are
// 1, so each augmentation carries one unit. Returns (flow, cost).
func (m *minCostFlow) minCostMaxFlow(s, t int) (int, float64) {
	var flow int
	var cost float64
	for {
		dist := make([]float64, m.n)
		prevEdge := make([]int, m.n)
		inQueue := make([]bool, m.n)
		for i := range dist {
			dist[i] = math.Inf(1)
			prevEdge[i] = -1
		}
		dist[s] = 0
		queue := []int{s}
		inQueue[s] = true
		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]
			inQueue[u] = false
			for _, ei := range m.adj[u] {
				e := m.edges[ei]
				if e.cap-e.flow > 0 && dist[u]+e.cost < dist[e.to]-1e-12 {
					dist[e.to] = dist[u] + e.cost
					prevEdge[e.to] = ei
					if !inQueue[e.to] {
						queue = append(queue, e.to)
						inQueue[e.to] = true
					}
				}
			}
		}
		if prevEdge[t] == -1 {
			break
		}
		for v := t; v != s; {
			ei := prevEdge[v]
			m.edges[ei].flow++
			m.edges[ei^1].flow--
			v = m.edges[ei^1].to
		}
		flow++
		cost += dist[t]
	}
	return flow, cost
}
