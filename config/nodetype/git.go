package nodetype

import (
	"fmt"
	"path/filepath"
)

type gitOpts struct {
	URL        string `json:"url"`         // the repo URL
	SubDir     string `json:"sub-dir"`     // the sub dir to check into
	Branch     string `json:"branch"`      // what to checkout
	Hash       string `json:"hash"`        // the exact hash for repeatability
	FromBranch string `json:"from-branch"` // what to checkout and rebase onto Ref
	GitKey     string `json:"git-key"`     // what key file to use
}

// gitMerge is an executable node that checks out a hash and then
// checks out another - and then merges into it from the other
type gitMerge struct{}

func (g gitMerge) Match(ol, or Opts) bool {
	return true
}

func (g gitMerge) Execute(ws *Workspace, in Opts, output chan string) (int, Opts, error) {

	gop := gitOpts{}
	err := decode(in, &gop)
	if err != nil {
		return 255, nil, err
	}

	if gop.URL == "" {
		return 255, nil, fmt.Errorf("problem getting git url option")
	}
	if gop.Branch == "" {
		return 255, nil, fmt.Errorf("problem getting ref option")
	}
	if gop.FromBranch == "" {
		return 255, nil, fmt.Errorf("problem getting from ref option")
	}

	output <- "git checkout: " + gop.URL + " merge into: " + gop.Branch + " from: " + gop.FromBranch

	return 0, nil, nil
}

// gitCheckout checks out a has from a url
type gitCheckout struct{}

func (g gitCheckout) Match(ol, or Opts) bool {
	return true
}

func (g gitCheckout) Execute(ws *Workspace, in Opts, output chan string) (int, Opts, error) {
	gop := gitOpts{}
	err := decode(in, &gop)
	if err != nil {
		return 255, nil, err
	}
	if gop.Branch == "" {
		return 255, nil, fmt.Errorf("problem getting branch option")
	}
	if gop.URL == "" {
		return 255, nil, fmt.Errorf("problem getting git url option")
	}

	// for testing
	if gop.URL == "git@github.com:floeit/floe-test.git" {
		output <- "in dir: /Users/Dan/.flow/spaces/danmux/ws/h1-12/src/github.com/floeit"
		output <- "git clone --branch master --depth 1 git@github.com:floeit/floe-test.git"
		output <- "Cloning into 'floe'..."
		return 0, nil, nil
	}
	var env []string
	if gop.GitKey != "" {
		env = []string{fmt.Sprintf(`GIT_SSH_COMMAND=ssh -i %s`, gop.GitKey)}
	}
	// git clone --branch mytag0.1 --depth 1 https://example.com/my/repo.git
	args := []string{"clone", "--branch", gop.Branch, "--depth", "1", gop.URL}
	status := doRun(filepath.Join(ws.BasePath, gop.SubDir), env, output, "git", args...)

	return status, nil, nil
}
