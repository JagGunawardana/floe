package git

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/floeit/floe/exe"
)

// Ref contains details of a git reference
type Ref struct {
	Name   string
	Type   string
	Hash   string
	Change time.Time
}

// Hashes stores the result of a GitLS
type Hashes struct {
	RepoURL string
	Hashes  map[string]Ref
}

// Ls list a remote repo
func Ls(url, pattern, exclude, gitKey string) (*Hashes, []string, bool) {
	if pattern == "" {
		pattern = "refs/*"
	}

	var env []string
	if gitKey != "" {
		env = []string{fmt.Sprintf(`GIT_SSH_COMMAND=ssh -i %s`, gitKey)}
	}

	gitOut, status := exe.RunOutput(env, "", "git", "ls-remote", "--refs", url, pattern)
	if status != 0 {
		return nil, gitOut, false
	}

	latestHash := &Hashes{
		RepoURL: url,
	}
	// drop the command and blank line
	parseGitResponse(gitOut[2:], latestHash, exclude)
	return latestHash, gitOut, true
}

func parseGitResponse(lines []string, hashes *Hashes, exclude string) {
	exclude = strings.TrimSpace(exclude)
	excl, _ := regexp.Compile(exclude)

	// map the lines by branch
	hashes.Hashes = map[string]Ref{}
	now := time.Now().UTC()
	for _, l := range lines { // from 2 onwards 1 = command 0 = empty
		sl := strings.Fields(l)

		if len(sl) < 2 {
			continue
		}

		dp := strings.Split(sl[1], "/")
		if len(dp) < 3 || dp[0] != "refs" {
			continue
		}

		if exclude != "" && excl.MatchString(sl[1]) {
			continue
		}

		ty := "branch"
		switch {
		case strings.HasPrefix(dp[1], "pull"):
			ty = "pull"
		case strings.HasPrefix(dp[1], "tag"):
			ty = "tag"
		}
		name := dp[2]
		name = strings.TrimSuffix(name, "^{}")
		hashes.Hashes[sl[1]] = Ref{
			Name:   name,
			Type:   ty,
			Hash:   sl[0],
			Change: now,
		}
	}
}
