package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRootCmd(t *testing.T) {
	testRootPath := "/tmp/dirupdate"

	if err := os.Mkdir(testRootPath, 0755); err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(testRootPath)

	createRepos(t, testRootPath)

	startTime := time.Now()

	cmd := newRootCmd()

	cmd.SetArgs([]string{
		fmt.Sprintf("--root=%s", testRootPath),
		"--stash-changes",
		"-d",
	})

	err := cmd.Execute()
	if err != nil {
		t.Error(err)
	}

	repositories := []string{"no-change", "changed", "dirty"}

	for _, repo := range repositories {
		info, err := os.Stat(filepath.Join(testRootPath, repo))
		if err != nil {
			t.Error(err)
		}

		switch repo {
		case "no-change":
			if info.ModTime().After(startTime) {
				t.Errorf("repository %s was updated", repo)
			}
		case "changed":
			if !info.ModTime().After(startTime) {
				t.Errorf("repository %s was updated", repo)
			}
		case "dirty":
			if !info.ModTime().After(startTime) {
				t.Errorf("repository %s was updated", repo)
			}

			cmdStash := exec.Command("git", "stash", "list")
			cmdStash.Dir = filepath.Join(testRootPath, repo)

			out, err := cmdStash.Output()

			if err != nil {
				t.Error(err)
			}

			if !strings.Contains(string(out), "dirupdate") {
				t.Errorf("repository %s was not stashed properly", repo)
			}
		}
	}
}

func createRepos(t *testing.T, rootPath string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(rootPath, "no-remote/.git"), 0755); err != nil {
		t.Fatal(err)
	}

	cmdNoChangeRepo := exec.Command("git", "clone", "https://github.com/KDE/dummy.git", "no-change")
	cmdNoChangeRepo.Dir = rootPath
	if err := cmdNoChangeRepo.Run(); err != nil {
		t.Fatal(err)
	}

	cmdChangedRepo := exec.Command("cp", "-R", "no-change", "changed")
	cmdChangedRepo.Dir = rootPath
	if err := cmdChangedRepo.Run(); err != nil {
		t.Fatal(err)
	}
	cmdReset := exec.Command("git", "reset", "--hard", "HEAD^")
	cmdReset.Dir = filepath.Join(rootPath, "changed")
	if err := cmdReset.Run(); err != nil {
		t.Fatal(err)
	}

	cmdStashedRepo := exec.Command("cp", "-R", "no-change", "dirty")
	cmdStashedRepo.Dir = rootPath
	if err := cmdStashedRepo.Run(); err != nil {
		t.Fatal(err)
	}
	cmdDirty := exec.Command("touch", "x.tmp", "y.tmp")
	cmdDirty.Dir = filepath.Join(rootPath, "dirty")
	if err := cmdDirty.Run(); err != nil {
		t.Fatal(err)
	}
}
