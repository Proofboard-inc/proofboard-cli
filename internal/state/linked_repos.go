package state

import "github.com/proofboard/proofboard/internal/model"

func AddLinkedRepo(state model.State, repo model.LinkedRepoState) model.State {
	if state.LinkedRepos == nil {
		state.LinkedRepos = make(map[string]model.LinkedRepoState)
	}
	state.LinkedRepos[repo.RepoHash] = repo
	return state
}

func RemoveLinkedRepo(state model.State, repoHash string) model.State {
	delete(state.LinkedRepos, repoHash)
	return state
}
