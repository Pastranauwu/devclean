package config

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// RepoRoot returns the top-level directory of the git repository
// containing dir.
func RepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("no hay repositorio git aquí · corre git init y reintenta")
	}
	return strings.TrimSpace(string(out)), nil
}

// DetectBaseBranch finds the base branch of the repository at root:
// the remote HEAD if set, else main or master if they exist, else the
// current branch. Empty string if nothing can be determined.
func DetectBaseBranch(root string) string {
	if ref, err := gitOut(root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimPrefix(ref, "origin/")
	}
	for _, candidate := range []string{"main", "master"} {
		if _, err := gitOut(root, "show-ref", "--verify", "--quiet", "refs/heads/"+candidate); err == nil {
			return candidate
		}
	}
	if branch, err := gitOut(root, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
		return branch
	}
	return ""
}

// DetectTestCommand infers the project test command from well-known
// manifest files, in priority order: package.json with a test script,
// Makefile with a test target, go.mod, pyproject.toml.
func DetectTestCommand(root string) (string, bool) {
	if hasNodeTestScript(root) {
		return "npm test", true
	}
	if hasMakeTestTarget(root) {
		return "make test", true
	}
	if fileExists(filepath.Join(root, "go.mod")) {
		return "go test ./...", true
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) {
		return "pytest", true
	}
	return "", false
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hasNodeTestScript(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	return strings.TrimSpace(pkg.Scripts["test"]) != ""
}

var makeTestTarget = regexp.MustCompile(`^test\s*:`)

func hasMakeTestTarget(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if makeTestTarget.MatchString(line) {
			return true
		}
	}
	return false
}
