package sandbox

import "fmt"

// CloneScript returns a shell script that authenticates with GitHub and clones
// the given repo into workDir. If branch is non-empty the script checks out
// that branch after cloning.
func CloneScript(repo, branch, workDir string) (string, error) {
	if repo == "" {
		return "", fmt.Errorf("repo must not be empty")
	}
	script := `set -e
gh auth setup-git
git config --global user.name "${GODARK_GIT_AUTHOR_NAME:-dark-factory}"
git config --global user.email "${GODARK_GIT_AUTHOR_EMAIL:-dark-factory@noreply}"
` + fmt.Sprintf("git clone https://github.com/%s.git %s\n", repo, workDir)

	if branch != "" {
		script += fmt.Sprintf("cd %s && git checkout %s\n", workDir, branch)
	}

	return script, nil
}

// EntrypointScript returns a complete shell script that runs the clone step
// followed by the given agent command.
func EntrypointScript(cloneScript, agentCmd string) string {
	return "#!/bin/sh\nset -e\n" + cloneScript + agentCmd + "\n"
}
