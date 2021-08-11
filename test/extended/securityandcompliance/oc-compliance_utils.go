package securityandcompliance

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"runtime/debug"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests-private/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// CLI provides function to call the CLI
type CLI struct {
	execPath        string
	ExecCommandPath string
	verb            string
	username        string
	globalArgs      []string
	commandArgs     []string
	finalArgs       []string
	stdin           *bytes.Buffer
	stdout          io.Writer
	stderr          io.Writer
	verbose         bool
	showInfo        bool
	skipTLS         bool
}

//  initialize the OC-Compliance framework
func OcComplianceCLI() *CLI {
	ocPlug := &CLI{}
	ocPlug.execPath = "oc-compliance"
	ocPlug.showInfo = true
	return ocPlug
}

// Run executes given oc-compliance command
func (c *CLI) Run(commands ...string) *CLI {
	in, out, errout := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	ocPlug := &CLI{
		execPath:        c.execPath,
		ExecCommandPath: c.ExecCommandPath,
	}
	ocPlug.globalArgs = commands
	ocPlug.stdin, ocPlug.stdout, ocPlug.stderr = in, out, errout
	return ocPlug.setOutput(c.stdout)
}

// setOutput allows to override the default command output
func (c *CLI) setOutput(out io.Writer) *CLI {
	c.stdout = out
	return c
}

// Args sets the additional arguments for the oc-compliance CLI command
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
		e2e.Logf("DEBUG: %s %s\n", c.execPath, c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	if c.ExecCommandPath != "" {
		e2e.Logf("set exec command path is %s\n", c.ExecCommandPath)
		cmd.Dir = c.ExecCommandPath
	}
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

func assertCheckProfileControls(oc *exutil.CLI, profl string, keyword [2]string) {
	var kw string
	var flag bool = true
	proControl, err := OcComplianceCLI().Run("controls").Args("profile", profl, "-n", oc.Namespace()).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, v := range keyword {
		kw = fmt.Sprintf("%s", v)
		if !strings.Contains(proControl, kw) {
			e2e.Failf("The keyword %v not exist!", v)
			flag = false
			break
		} else {
			e2e.Logf("keyword matches '%v' with profile '%v' standards and controls", v, profl)
		}
	}
	if flag == false {
		e2e.Failf("The keyword not exist!")
	}
}

func assertRuleResult(oc *exutil.CLI, rule string, namespace string, keyword [2]string) {
	var kw string
	var flag bool = true
	viewResult, err := OcComplianceCLI().Run("view-result").Args(rule, "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, v := range keyword {
		kw = fmt.Sprintf("%s", v)
		if !strings.Contains(viewResult, kw) {
			e2e.Failf("The keyword '%v' not exist!", v)
			flag = false
			break
		} else {
			e2e.Logf("keyword matches '%v' with view-result report output", v)
		}
	}
	if flag == false {
		e2e.Failf("The keyword not exist!")
	}
}

func assertDryRunBind(oc *exutil.CLI, profile string, namespace string, keyword string) {
	cisPrfl, err := OcComplianceCLI().Run("bind").Args("--dry-run", "-N", "my-binding", profile, "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if !strings.Contains(cisPrfl, keyword) {
		e2e.Failf("The keyword '%v' not exist!", keyword)
	} else {
		e2e.Logf("keyword matches '%v' with bind dry run command output", keyword)
	}
}
