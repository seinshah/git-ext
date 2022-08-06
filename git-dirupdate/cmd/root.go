package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	warnThreshold = 100
)

var (
	// rootCmd represents the base command when called without any subcommands
	rootCmd = newRootCmd()
)

var (
	pathPrefix   *string
	branches     *[]string
	allBranches  *bool
	stashChanges *bool
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git-dirupdate",
		Short: "git extension ",
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := expandPathWithTilde(*pathPrefix)
			if err != nil {
				return err
			}

			repositories, err, consent := findRepositories(rootDir)
			if err != nil {
				return err
			} else if !consent {
				return nil
			}

			for _, repository := range repositories {
				if err = updateRepository(repository); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(
		pathPrefix, "root", "r", os.Getenv("GIT_DIRCLONE_ROOT_DIR"),
		"root directory. default is environment variable GIT_DIRCLONE_ROOT_DIR")

	cmd.PersistentFlags().StringSliceVarP(
		branches, "branch", "b", []string{"main", "master"},
		"comma-separated branches to update in each repository. default is master,main")

	cmd.PersistentFlags().BoolVarP(
		allBranches, "all-branches", "a", false,
		"whether to update all branches of each repository or not. if true, --branch is ignored. default is false")

	cmd.PersistentFlags().BoolVarP(
		stashChanges, "stash-changes", "s", false,
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

func findRepositories(rootDir string) ([]string, error, bool) {
	if rootDir == "" {
		rootDir = "."
	}

	found, err := exec.Command("find", "-Ls", strings.TrimSuffix(rootDir, "/"), "-type", "d", "-name", ".git").Output()

	if err != nil {
		return nil, err, false
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

		return nil, nil, false
	}

	if len(repositories) > warnThreshold {
		fmt.Printf("Found %d repositories in %s, do you want to continue? [no/yes]\n", len(repositories), rootDir)

		var answer string
		if _, err = fmt.Scanln(&answer); err != nil {
			return nil, err, false
		}

		if strings.Trim(strings.ToLower(answer), " ") != "yes" {
			return nil, nil, false
		}
	}

	return repositories, nil, true
}

func updateRepository(repository string) error {
	repo := newRepository(repository)

	fmt.Println(repository)

	dirty, err := repo.IsDirty()
	if err != nil {
		return err
	} else if dirty {
		if *stashChanges {
			if err := repo.Stash(); err != nil {
				return err
			}
			fmt.Println("\tstashed changes")
		} else {
			fmt.Println("\tdirty, skipping")

			return nil
		}
	}

	fetchCmd := exec.Command("git", "fetch", "--all")
	fetchCmd.Dir = repository
	if err = fetchCmd.Run(); err != nil {
		return err
	}

	activeBranches := *branches
	if *allBranches {
		if activeBranches, err = repo.GetAllBranches(); err != nil {
			return err
		}
	}

	fmt.Printf("\tupdating %d branches\n", len(activeBranches))

	return repo.Update(activeBranches)
}
