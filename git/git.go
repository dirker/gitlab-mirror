package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Exec executes a git command and returns the status and combined output
func Exec(arg ...string) (output string, err error) {
	fmt.Printf("[git] %s\n", strings.Join(arg, " "))
	cmd := exec.Command("git", arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return
}

func ExecIn(repoPath string, arg ...string) (output string, err error) {
	a := []string{"--git-dir", repoPath}
	a = append(a, arg...)

	return Exec(a...)
}
