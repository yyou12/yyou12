package operatorsdk

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"runtime/debug"
	"strings"

	g "github.com/onsi/ginkgo"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// CLI provides function to call the Operator-sdk CLI
type CLI struct {
	execPath	string
	verb		string
	username	string
	globalArgs	[]string
	commandArgs	[]string
	finalArgs	[]string
	stdin		*bytes.Buffer
	stdout		io.Writer
	stderr		io.Writer
	verbose		bool
	showInfo	bool
	skipTLS		bool
}

// NewOperatorSDKCLI intialize the SDK framework
func NewOperatorSDKCLI() *CLI {
	client := &CLI{}
	client.username = "admin"
	client.execPath = "operator-sdk"
	client.showInfo = true
	return client
}

// Run executes given OperatorSDK command verb
func (c *CLI) Run(commands ...string) *CLI {
	in, out, errout := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	operatorsdk := &CLI{
		execPath: c.execPath,
		verb:     commands[0],
		username: c.username,
	}
	if c.skipTLS {
		operatorsdk.globalArgs = append([]string{"--skip-tls=true"}, commands...)
	} else {
		operatorsdk.globalArgs = commands
	}
	operatorsdk.stdin, operatorsdk.stdout, operatorsdk.stderr = in, out, errout
	return operatorsdk.setOutput(c.stdout)
}

// setOutput allows to override the default command output
func (c *CLI) setOutput(out io.Writer) *CLI {
	c.stdout = out
	return c
}

// Args sets the additional arguments for the OpenShift CLI command
func (c *CLI) Args(args ...string) *CLI {
	c.commandArgs = args
	c.finalArgs = append(c.globalArgs, c.commandArgs...)
	return c
}

func (c *CLI) printCmd() string {
	return strings.Join(c.finalArgs, " ")
}

// ExitError returns the error info
type ExitError struct {
	Cmd    string
	StdErr string
	*exec.ExitError
}

// FatalErr exits the test in case a fatal error has occurred.
func FatalErr(msg interface{}) {
	// the path that leads to this being called isn't always clear...
	fmt.Fprintln(g.GinkgoWriter, string(debug.Stack()))
	e2e.Failf("%v", msg)
}

// Output executes the command and returns stdout/stderr combined into one string
func (c *CLI) Output() (string, error) {
	if c.verbose {
		e2e.Logf("DEBUG: opm %s\n", c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	cmd.Stdin = c.stdin
	if c.showInfo {
		e2e.Logf("Running '%s %s'", c.execPath, strings.Join(c.finalArgs, " "))
	}
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	switch err.(type) {
	case nil:
		c.stdout = bytes.NewBuffer(out)
		return trimmed, nil
	case *exec.ExitError:
		e2e.Logf("Error running %v:\n%s", cmd, trimmed)
		return trimmed, &ExitError{ExitError: err.(*exec.ExitError), Cmd: c.execPath + " " + strings.Join(c.finalArgs, " "), StdErr: trimmed}
	default:
		FatalErr(fmt.Errorf("unable to execute %q: %v", c.execPath, err))
		// unreachable code
		return "", nil
	}
}
