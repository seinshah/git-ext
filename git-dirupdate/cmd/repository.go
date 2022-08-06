package cmd

import (
	"os/exec"
	"strings"
)

type repository struct {
	path          string
	remoteUpdated bool
}

func newRepository(path string) *repository {
	return &repository{path: path}
}

func (r *repository) IsDirty() (bool, error) {
	changesCmd := exec.Command("git", "status", "--porcelain")
	changesCmd.Dir = r.path
	changes, err := changesCmd.Output()

	if err != nil {
		return false, err
	}

	return len(changes) > 0, nil
}

func (r *repository) Stash() error {
	stashCmd := exec.Command("git", "stash")
	stashCmd.Dir = r.path

	return stashCmd.Run()
}

func (r *repository) GetAllBranches() ([]string, error) {
	if err := r.updateRemote(); err != nil {
		return nil, err
	}

	branchesCmd := exec.Command("git", "branch", "-l")
	branchesCmd.Dir = r.path

	branches, err := branchesCmd.Output()

	if err != nil {
		return nil, err
	}

	var branchesList []string
	for _, branch := range strings.Split(string(branches), "\n") {
		if branch == "" {
			continue
		}

		branchesList = append(branchesList, branch)
	}

	return branchesList, nil
}

func (r *repository) Update(brnaches []string) error {
	if err := r.updateRemote(); err != nil {
		return err
	}

	for _, branch := range brnaches {
		if err := r.updateBranch(branch); err != nil {
			return err
		}
	}

	return nil
}

func (r *repository) updateRemote() error {
	if r.remoteUpdated {
		return nil
	}

	remoteCmd := exec.Command("git", "fetch", "--all")
	remoteCmd.Dir = r.path

	return remoteCmd.Run()
}

func (r *repository) updateBranch(branch string) error {
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = r.path

	if err := checkoutCmd.Run(); err != nil {
		return err
	}

	pullCmd := exec.Command("git", "pull")
	pullCmd.Dir = r.path

	return pullCmd.Run()
}
