package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func gitBranch(dir string) (branch string, err error) {
	stdout, _, err := execute(gitCommand(dir, "rev-parse", "--abbrev-ref", "HEAD")...)
	if err != nil {
		return
	}
	branch = strings.TrimSpace(stdout)
	return
}

func hasBranch(dir string, branch string) (result bool, err error) {
	stdout, _, err := execute(gitCommand(dir, "branch", "-a")...)
	if err != nil {
		return
	}
	for _, line := range strings.Split(stdout, "\n") {
		if branchRegexp.MatchString(line) && strings.HasSuffix(line, branch) {
			result = true
			return
		}
	}
	return
}

var okFile = regexp.MustCompile("^[a-zA-Z0-9].*$")
var branchRegexp = regexp.MustCompile("^\\s*[a-zA-Z0-9/_-]+$")

func gitCommand(dir string, cmd ...string) []string {
	return append([]string{"git", fmt.Sprintf("--git-dir=%v/.git", dir), fmt.Sprintf("--work-tree=%v", dir)}, cmd...)
}

func gitExecute(dir string, cmd ...string) (err error) {
	return execute1(gitCommand(dir, cmd...)...)
}

func execute1(cmd ...string) (err error) {
	stdout, stderr, err := execute(cmd...)
	if err != nil {
		err = fmt.Errorf("When running %+v:\n%v\n%v\n%v", cmd, stdout, stderr, err)
	}
	return
}

func execute(cmd ...string) (stdoutString string, stderrString string, err error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := exec.Command(cmd[0], cmd[1:]...)
	command.Stdout, command.Stderr = stdout, stderr
	err = command.Run()
	stdoutString = stdout.String()
	stderrString = stderr.String()
	return
}

func checkout(base string, dir os.FileInfo, branch string, lock chan string) {
	abs := filepath.Join(base, dir.Name())
	if err := gitExecute(abs, "fetch"); err != nil {
		lock <- err.Error()
		return
	}
	has, err := hasBranch(abs, branch)
	if err != nil {
		lock <- err.Error()
	}
	if has {
		if err := gitExecute(abs, "checkout", branch); err != nil {
			lock <- err.Error()
			return
		}
		lock <- fmt.Sprintf("Checked out %#v in %#v", branch, abs)
	} else {
		if err := gitExecute(abs, "checkout", "master"); err != nil {
			lock <- err.Error()
			return
		}
		if err := gitExecute(abs, "pull"); err != nil {
			lock <- err.Error()
			return
		}
		lock <- fmt.Sprintf("Checked out \"master\" in %#v", abs)
	}
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	dir := flag.String("dir", cwd, "Where to look for repositories to checkout branches for.")
	curr, err := gitBranch(cwd)
	if err != nil {
		curr = "master"
	}
	branch := flag.String("branch", curr, "Which branch to checkout.")

	flag.Parse()

	dirFile, err := os.Open(*dir)
	if err != nil {
		panic(err)
	}
	children, err := dirFile.Readdir(-1)
	if err != nil {
		panic(err)
	}
	counter := 0
	lock := make(chan string)
	for _, child := range children {
		if child.IsDir() && okFile.MatchString(child.Name()) {
			counter++
			go checkout(*dir, child, *branch, lock)
		}
	}
	for i := 0; i < counter; i++ {
		fmt.Println(<-lock)
	}
}
