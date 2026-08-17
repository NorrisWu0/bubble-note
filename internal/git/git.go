package git

import (
	"os/exec"
	"regexp"
	"strings"
)

// Info is the minimal git state bubble-note surfaces in its footer.
type Info struct {
	IsRepo bool
	Branch string
	Dirty  bool
	Ahead  int
	Behind int
}

var (
	aheadRe  = regexp.MustCompile(`ahead (\d+)`)
	behindRe = regexp.MustCompile(`behind (\d+)`)
)

// IsRepo reports whether dir is inside a git work tree.
func IsRepo(dir string) bool {
	_, err := run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// Init initializes a git repository in dir.
func Init(dir string) error {
	return runErr(dir, "init", "-q")
}

// Status reports the current git status of dir, or a non-repo status if dir is
// not inside a repository.
func Status(dir string) (Info, error) {
	if !IsRepo(dir) {
		return Info{}, nil
	}
	porcelain, err := run(dir, "status", "--porcelain")
	if err != nil {
		return Info{}, err
	}
	branchLine, err := run(dir, "status", "-sb")
	if err != nil {
		return Info{}, err
	}
	status := Info{
		IsRepo: true,
		Branch: parseBranch(branchLine),
		Dirty:  strings.TrimSpace(porcelain) != "",
	}
	if match := aheadRe.FindStringSubmatch(branchLine); len(match) == 2 {
		status.Ahead = atoi(match[1])
	}
	if match := behindRe.FindStringSubmatch(branchLine); len(match) == 2 {
		status.Behind = atoi(match[1])
	}
	return status, nil
}

func parseBranch(branchLine string) string {
	line := strings.TrimSpace(branchLine)
	if !strings.HasPrefix(line, "## ") {
		return ""
	}
	rest := strings.TrimPrefix(line, "## ")
	if index := strings.Index(rest, "..."); index >= 0 {
		return rest[:index]
	}
	if index := strings.Index(rest, " ["); index >= 0 {
		return rest[:index]
	}
	return rest
}

func atoi(value string) int {
	var result int
	for _, r := range value {
		result = result*10 + int(r-'0')
	}
	return result
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func runErr(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}
