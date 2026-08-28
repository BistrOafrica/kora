package contract

import "errors"

// CommandBatch is a set of offline commands submitted for sync (OFFLINE-002).
type CommandBatch struct {
	Commands []CommandEnvelope `json:"commands"`
}

// ErrCausationCycle is returned when a command batch contains a causation
// cycle, which cannot be ordered deterministically.
var ErrCausationCycle = errors.New("offline command batch contains a causation cycle")

// OrderByCausation returns the commands topologically ordered by causation:
// a command whose CausationID references another command's ID in the batch is
// placed after its parent. Independent commands preserve their input order.
// This guarantees "parent before child" replay regardless of submission order.
func OrderByCausation(cmds []CommandEnvelope) ([]CommandEnvelope, error) {
	if len(cmds) < 2 {
		return cmds, nil
	}

	byID := make(map[string]int, len(cmds))
	for i, c := range cmds {
		if c.ID != "" {
			byID[c.ID] = i
		}
	}

	inDeg := make([]int, len(cmds))
	adj := make([][]int, len(cmds))
	for i, c := range cmds {
		if c.CausationID == "" {
			continue
		}
		if parent, ok := byID[c.CausationID]; ok {
			adj[parent] = append(adj[parent], i)
			inDeg[i]++
		}
	}

	var queue []int
	for i := range cmds {
		if inDeg[i] == 0 {
			queue = append(queue, i)
		}
	}

	ordered := make([]CommandEnvelope, 0, len(cmds))
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		ordered = append(ordered, cmds[i])
		for _, child := range adj[i] {
			inDeg[child]--
			if inDeg[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	if len(ordered) != len(cmds) {
		return nil, ErrCausationCycle
	}
	return ordered, nil
}
