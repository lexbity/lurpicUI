package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type CommandSpec struct {
	Path   string
	Args   []string
	Dir    string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type ProcessHandle interface {
	Kill() error
	Wait() error
}

type Runner interface {
	Run(spec CommandSpec) error
	Output(spec CommandSpec) ([]byte, error)
	Look(name string) (string, error)
}

type execRunner struct{}

func newExecRunner() *execRunner {
	return &execRunner{}
}

func (*execRunner) Run(spec CommandSpec) error {
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	cmd.Stdin = spec.Stdin
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	return cmd.Run()
}

func (*execRunner) Output(spec CommandSpec) ([]byte, error) {
	cmd := exec.Command(spec.Path, spec.Args...)
	cmd.Dir = spec.Dir
	if spec.Env != nil {
		cmd.Env = spec.Env
	}
	cmd.Stdin = spec.Stdin

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}

func (*execRunner) Look(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("executable %q not found on PATH: %w", name, err)
	}
	return path, nil
}

type CallMatcher func(CommandSpec) bool

func MatchCommand(path string, args ...string) CallMatcher {
	return func(spec CommandSpec) bool {
		if spec.Path != path {
			return false
		}
		if len(args) == 0 {
			return true
		}
		if len(spec.Args) != len(args) {
			return false
		}
		for i := range args {
			if spec.Args[i] != args[i] {
				return false
			}
		}
		return true
	}
}

type fakeRunner struct {
	mu      sync.Mutex
	calls   []CommandSpec
	results []cannedResult
}

type cannedResult struct {
	matcher CallMatcher
	stdout  []byte
	stderr  []byte
	err     error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{}
}

func (f *fakeRunner) When(matcher CallMatcher) *fakeResultBuilder {
	return &fakeResultBuilder{
		runner:  f,
		matcher: matcher,
	}
}

type fakeResultBuilder struct {
	runner  *fakeRunner
	matcher CallMatcher
}

func (b *fakeResultBuilder) Then(stdout, stderr string, err error) {
	b.runner.mu.Lock()
	defer b.runner.mu.Unlock()
	b.runner.results = append(b.runner.results, cannedResult{
		matcher: b.matcher,
		stdout:  []byte(stdout),
		stderr:  []byte(stderr),
		err:     err,
	})
}

func (f *fakeRunner) Run(spec CommandSpec) error {
	f.mu.Lock()
	f.calls = append(f.calls, spec)
	result := f.match(spec)
	f.mu.Unlock()

	if spec.Stdout != nil && len(result.stdout) > 0 {
		_, _ = spec.Stdout.Write(result.stdout)
	}
	if spec.Stderr != nil && len(result.stderr) > 0 {
		_, _ = spec.Stderr.Write(result.stderr)
	}
	return result.err
}

func (f *fakeRunner) Output(spec CommandSpec) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, spec)
	result := f.match(spec)
	f.mu.Unlock()

	var buf bytes.Buffer
	if len(result.stdout) > 0 {
		buf.Write(result.stdout)
	}
	if len(result.stderr) > 0 {
		buf.Write(result.stderr)
	}
	if result.err != nil {
		return buf.Bytes(), result.err
	}
	return buf.Bytes(), nil
}

func (f *fakeRunner) Look(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("executable %q not found on PATH: %w", name, err)
	}
	return path, nil
}

func (f *fakeRunner) Calls() []CommandSpec {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]CommandSpec, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeRunner) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeRunner) match(spec CommandSpec) cannedResult {
	for i := len(f.results) - 1; i >= 0; i-- {
		r := f.results[i]
		if r.matcher(spec) {
			return r
		}
	}
	return cannedResult{
		err: errors.New("fakeRunner: no matching result registered"),
	}
}
