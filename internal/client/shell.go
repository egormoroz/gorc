package client

import (
	"bufio"
	"os/exec"
)

type Shell struct {
	cmd    *exec.Cmd
	stdin  chan string
	stdout chan string
	done   chan struct{}
}

func NewShell() (*Shell, error) {
	cmd := exec.Command("powershell.exe")

	stdin, err := cmd.StdinPipe()
	if err != nil { return nil, err }

	stdout, err := cmd.StdoutPipe()
	if err != nil { return nil, err }

	stderr, err := cmd.StderrPipe()
	if err != nil { return nil, err }

	err = cmd.Start()
	if err != nil { return nil, err }

	shell := &Shell{
		cmd:    cmd,
		stdin:  make(chan string),
		stdout: make(chan string, 32), // Buffered channel
		done:   make(chan struct{}),
	}

	// Handle stdin
	go func() {
		for input := range shell.stdin {
			stdin.Write([]byte(input + "\n"))
		}
	}()

	// Handle stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case shell.stdout <- scanner.Text():
			case <-shell.done: return
			}
		}
        close(shell.stdout)
	}()

	// Handle stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			select {
			case shell.stdout <- scanner.Text():
			case <-shell.done: return
			}
		}
	}()

	return shell, nil
}

func (s *Shell) Execute(command string) {
	s.stdin <- command
}

func (s *Shell) Close() error {
	close(s.stdin)
	close(s.done)
	return s.cmd.Process.Kill()
}
