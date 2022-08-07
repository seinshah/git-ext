package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

const (
	warnThreshold = 10
)

var (
	errStashNotAllowed = fmt.Errorf("repository is dirty and stashing is not allowed")
	errNoBranch        = fmt.Errorf("no branch to update")
)

var (
	// rootCmd represents the base command when called without any subcommands
	rootCmd = newRootCmd()
)

var (
	pathPrefix        string
	requestedBranches []string
	allBranches       bool
	stashChanges      bool
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git-dirupdate",
		Short: "git extension ",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := expandPathWithTilde(pathPrefix)
			if err != nil {
				return err
			}

			spinner, err := pterm.DefaultSpinner.WithRemoveWhenDone(true).Start("Finding repositories")
			if err != nil {
				return err
			}

			repositories, err := findRepositories(rootDir)
			if err != nil {
				return err
			}

			spinner.Success("Found %d repositories", len(repositories))

			if len(repositories) > warnThreshold {
				ok, err := pterm.DefaultInteractiveConfirm.
					WithDefaultText(fmt.Sprintf("Are you sure you want to update %d repositories?", len(repositories))).
					Show()

				if err != nil || !ok {
					return err
				}
			}

			for _, repo := range repositories {
				updateRepository(repo) // nolint: errcheck
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(
		&pathPrefix, "root", "r", os.Getenv("GIT_DIRCLONE_ROOT_DIR"),
		"root directory. default is environment variable GIT_DIRCLONE_ROOT_DIR")

	cmd.PersistentFlags().StringSliceVarP(
		&requestedBranches, "branch", "b", []string{"main", "master"},
		"comma-separated branches to update in each repository. default is master,main")

	cmd.PersistentFlags().BoolVarP(
		&allBranches, "all-branches", "a", false,
		"whether to update all branches of each repository or not. if true, --branch is ignored. default is false")

	cmd.PersistentFlags().BoolVarP(
		&stashChanges, "stash-changes", "s", false,
		"when a branch is dirty, if this is true, changes will be stashed and then updated. default is false")

	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func expandPathWithTilde(rootDir string) (string, error) {

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	dir := usr.HomeDir

	if rootDir == "~" {
		// In case of "~", which won't be caught by the "else if"
		rootDir = dir
	} else if strings.HasPrefix(rootDir, "~/") {
		// Use strings.HasPrefix so we don't match paths like
		// "/something/~/something/"
		rootDir = filepath.Join(dir, rootDir[2:])
	}

	return rootDir, nil
}

func findRepositories(rootDir string) ([]string, error) {
	if rootDir == "" {
		rootDir = "."
	}

	found, err := exec.Command("find", "-Ls", strings.TrimSuffix(rootDir, "/"), "-type", "d", "-name", ".git").Output()

	if err != nil {
		return nil, err
	}

	var repositories []string

	for _, path := range strings.Split(string(found), "\n") {
		if !strings.HasSuffix(path, ".git") {
			continue
		}

		repositories = append(repositories, strings.TrimSuffix(path, "/.git"))
	}

	if len(repositories) == 0 {
		fmt.Printf("No repositories found in %s\n", rootDir)

		return nil, nil
	}

	return repositories, nil
}

func updateRepository(repository string) error {
	viz, err := pterm.DefaultSpinner.Start(repository)
	if err != nil {
		return err
	}

	if err = stashIfDirty(repository); err != nil {
		if errors.Is(err, errStashNotAllowed) {
			viz.InfoPrinter = &pterm.PrefixPrinter{
				Prefix: pterm.Prefix{
					Style: &pterm.Style{pterm.FgBlack, pterm.BgLightBlue},
					Text:  "SKIPPED",
				},
			}
			viz.Info()
		} else {
			viz.Fail()
		}

		return err
	}

	activeBranches, err := fetchBranchesToUpdate(repository)
	if err != nil {
		if errors.Is(err, errNoBranch) {
			viz.InfoPrinter = &pterm.PrefixPrinter{
				Prefix: pterm.Prefix{
					Style: &pterm.Style{pterm.FgYellow, pterm.BgDarkGray},
					Text:  " NO-BRANCH ",
				},
			}
			viz.Info()
		} else {
			viz.Fail()
		}

		return err
	}

	var failedUpdates []string

	for _, branch := range activeBranches {
		viz.UpdateText(fmt.Sprintf("(%s): %s", branch, repository))

		if err := updateBranch(repository, branch); err != nil {
			failedUpdates = append(failedUpdates, branch)
		}
	}

	if len(failedUpdates) == len(activeBranches) {
		viz.Fail()
	} else if len(failedUpdates) > 0 {
		viz.Warning(fmt.Sprintf("%s: [%d/%d] (%s)", repository, len(activeBranches)-len(failedUpdates),
			len(activeBranches), strings.Join(failedUpdates, ", ")))
	} else {
		viz.Success(fmt.Sprintf("%s: [%d/%d]", repository, len(activeBranches), len(activeBranches)))
	}

	return nil
}

func stashIfDirty(repo string) error {
	changesCmd := exec.Command("git", "status", "--porcelain")
	changesCmd.Dir = repo
	changes, err := changesCmd.Output()

	if err != nil {
		return err
	}

	if len(changes) == 0 {
		return nil
	}

	if !stashChanges {
		return errStashNotAllowed
	}

	stashCmd := exec.Command("git", "stash")
	stashCmd.Dir = repo

	return stashCmd.Run()
}

func fetchBranchesToUpdate(repo string) ([]string, error) {
	remoteCmd := exec.Command("git", "fetch", "--all")
	remoteCmd.Dir = repo

	if err := remoteCmd.Run(); err != nil {
		return nil, err
	}

	branchesCmd := exec.Command("git", "branch", "-l", "--format=%(refname:short)")
	branchesCmd.Dir = repo

	branches, err := branchesCmd.Output()

	if err != nil {
		return nil, err
	}

	var branchesList []string
	for _, branch := range strings.Split(string(branches), "\n") {
		if branch == "" ||
			strings.HasPrefix(branch, "refs/") ||
			strings.HasPrefix(branch, "heads/") ||
			strings.HasPrefix(branch, "origin/") ||
			strings.Contains(branch, " ") {
			continue
		}

		if !allBranches {
			for _, activeBranch := range requestedBranches {
				if branch == activeBranch {
					branchesList = append(branchesList, branch)
				}
			}
		} else {
			branchesList = append(branchesList, branch)
		}
	}

	if len(branchesList) == 0 {
		return nil, errNoBranch
	}

	return branchesList, nil
}

func updateBranch(repo string, branch string) error {
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = repo

	if err := checkoutCmd.Run(); err != nil {
		return err
	}

	pullCmd := exec.Command("git", "pull")
	pullCmd.Dir = repo

	return pullCmd.Run()
}
