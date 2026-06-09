package state

import "github.com/proofboard/proofboard/internal/model"

func SetWatchedBranches(state model.State, branches []string) model.State {
	state.WatchedBranches = append([]string(nil), branches...)
	return state
}
