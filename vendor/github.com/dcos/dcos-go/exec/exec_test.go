package exec

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func getDefaultShellPath() string {
	switch runtime.GOOS {
	case "windows":
		return "powershell.exe"
	default:
		return "/bin/bash"
	}
}

func getFixture(name string) string {
	switch runtime.GOOS {
	case "windows":
		return "fixture/" + name + ".ps1"
	default:
		return "fixture/" + name + ".sh"
	}
}

func getLsCmd() string {
	switch runtime.GOOS {
	case "windows":
		return getDefaultShellPath()
	default:
		return "ls"
	}
}

func getLsParams() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"Get-ChildItem"}
	default:
		return []string{"-la"}
	}
}

func TestRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ce, err := Run(ctx, getLsCmd(), getLsParams())
	if err != nil {
		t.Fatal(err)
	}

	buffer := new(bytes.Buffer)
	io.Copy(buffer, ce)
	err = <-ce.Done
	if err != nil {
		t.Fatalf("Return should be nil. Got %s", err)
	}

	debugOutput := buffer.String()
	scanner := bufio.NewScanner(buffer)
	var foundString int
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "exec.go") || strings.Contains(scanner.Text(), "exec_test.go") {
			foundString++
		}
	}

	if foundString != 2 {
		t.Fatalf("Expecting `exec.go` and `exec_test.go` in output. Got: %s", debugOutput)
	}
}

func TestRunTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ce, err := Run(ctx, getDefaultShellPath(), []string{getFixture("infinite")})
	if err != nil {
		t.Fatal(err)
	}
	buffer := new(bytes.Buffer)
	io.Copy(buffer, ce)
	err = <-ce.Done
	if err != context.DeadlineExceeded {
		t.Fatalf("Return should be %s. Got %s", context.DeadlineExceeded, err)
	}
}

func TestRunCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	ce, err := Run(ctx, getDefaultShellPath(), []string{getFixture("infinite")})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(time.Second)
		cancel()
	}()
	buffer := new(bytes.Buffer)
	io.Copy(buffer, ce)
	err = <-ce.Done
	if err != context.Canceled {
		t.Fatalf("Expected %s .Got %s", context.Canceled, err)
	}
}

func TestBadReturnCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ce, err := Run(ctx, "command_no_found", []string{"abc"})
	if err != nil {
		t.Fatal(err)
	}
	buffer := new(bytes.Buffer)
	io.Copy(buffer, ce)
	err = <-ce.Done
	if !strings.Contains(err.Error(), "command_no_found") {
		t.Fatalf("Expected `command_no_found` in error output.Got: %s", err)
	}
}

func getEchoCommand() string {
	switch runtime.GOOS {
	case "windows":
		return getDefaultShellPath()
	default:
		return "echo"
	}
}

func getEchoCommandParameters() string {
	switch runtime.GOOS {
	case "windows":
		return "write-host hello"
	default:
		return "hello"
	}
}

func TestSimpleFullOutput(t *testing.T) {
	stdout, stderr, code, err := SimpleFullOutput(time.Second*10, getEchoCommand(), getEchoCommandParameters())
	if err != nil {
		t.Fatal(err)
	}

	stdoutStr := string(stdout)
	stderrStr := string(stderr)

	if stdoutStr != "hello\n" {
		t.Fatalf("expect output hello. Got %s", stdoutStr)
	}

	if stderrStr != "" {
		t.Fatalf("expect empty stderr. Got %s", stderrStr)
	}

	if code != 0 {
		t.Fatalf("expect exit code 0. Got %d", code)
	}

	// Expect error
	stdout, stderr, code, err = SimpleFullOutput(time.Second*10, "ec", "hello")
	if err == nil {
		t.Fatal("expect error got nil")
	}

	stdoutStr = string(stdout)
	stderrStr = string(stderr)

	if stdoutStr != "" || stderrStr != "" {
		t.Fatalf("stdout and stderr must be empty. Got %s, %s", stdoutStr, stderrStr)
	}

	if code != 0 {
		t.Fatalf("expect exit code 0. Got %d", code)
	}
}

func getSleepCommand() string {
	switch runtime.GOOS {
	case "windows":
		return getDefaultShellPath()
	default:
		return "sleep"
	}
}

func getSleepParameters(sleep int) string {
	switch runtime.GOOS {
	case "windows":
		return "start-sleep " + strconv.FormatInt(int64(sleep), 10)
	default:
		return strconv.FormatInt(int64(sleep), 10)
	}
}

func TestSimpleFullOutputTimeout(t *testing.T) {
	_, _, _, err := SimpleFullOutput(time.Microsecond*100, getSleepCommand(), getSleepParameters(10))
	if err == nil {
		t.Fatal("expect error got nil")
	}
}

func TestSimpleFullOutputTimeoutPass(t *testing.T) {
	_, _, _, err := SimpleFullOutput(time.Second*10, getSleepCommand(), getSleepParameters(1))
	if err != nil {
		t.Fatalf("expect nil error. Got %s", err)
	}
}

func TestReturnCode(t *testing.T) {
	_, _, code, err := SimpleFullOutput(time.Second*10, getDefaultShellPath(), getFixture("return-err"))
	if err != nil {
		t.Fatalf("expect nil error. Got %s", err)
	}

	if code != 10 {
		t.Fatalf("expect return code 10. Got %d", code)
	}
}
