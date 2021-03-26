package container

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// contains check list contain one string
func contains(s []string, e string) bool {
	for _, a := range s {
		if strings.Contains(a, e) {
			return true
		}
	}
	return false
}

// DockerCLI provides function to run the docker command
type DockerCLI struct {
	CLI             *client.Client
	execPath        string
	execCommandPath string
	globalArgs      []string
	commandArgs     []string
	finalArgs       []string
	verbose         bool
	stdin           *bytes.Buffer
	stdout          io.Writer
	stderr          io.Writer
	showInfo        bool
}

// NewDockerCLI initialize the docker cli framework
func NewDockerCLI() *DockerCLI {
	newclient := &DockerCLI{}
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		e2e.Failf("get docker client failed")
	}
	newclient.CLI = cli
	newclient.execPath = "docker"
	newclient.showInfo = true
	return newclient
}

// Run executes given docker command
func (c *DockerCLI) Run(commands ...string) *DockerCLI {
	in, out, errout := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	docker := &DockerCLI{
		execPath:        c.execPath,
		execCommandPath: c.execCommandPath,
	}
	docker.globalArgs = commands
	docker.stdin, docker.stdout, docker.stderr = in, out, errout
	return docker.setOutput(c.stdout)
}

// setOutput allows to override the default command output
func (c *DockerCLI) setOutput(out io.Writer) *DockerCLI {
	c.stdout = out
	return c
}

// Args sets the additional arguments for the docker CLI command
func (c *DockerCLI) Args(args ...string) *DockerCLI {
	c.commandArgs = args
	c.finalArgs = append(c.globalArgs, c.commandArgs...)
	return c
}

func (c *DockerCLI) printCmd() string {
	return strings.Join(c.finalArgs, " ")
}

// Output executes the command and returns stdout/stderr combined into one string
func (c *DockerCLI) Output() (string, error) {
	if c.verbose {
		e2e.Logf("DEBUG: docker %s\n", c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	if c.execCommandPath != "" {
		e2e.Logf("set exec command path is %s\n", c.execCommandPath)
		cmd.Dir = c.execCommandPath
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

// GetImageID is to get the image ID by image tag
func (c *DockerCLI) GetImageID(imageTag string) (string, error) {
	imageID := ""
	ctx := context.Background()
	images, err := c.CLI.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		e2e.Logf("get docker image list failed")
		return imageID, err
	}
	for _, image := range images {
		if strings.Contains(strings.Join(image.RepoTags, ","), imageTag) {
			e2e.Logf("image ID is %s\n", image.ID)
			return image.ID, nil
		}
	}
	return imageID, nil
}

// RemoveImage is to remove image
func (c *DockerCLI) RemoveImage(imageIndex string) (bool, error) {
	imageID, err := c.GetImageID(imageIndex)
	if err != nil {
		return false, err
	}
	e2e.Logf("%s imageID is %s\n", imageIndex, imageID)
	ctx := context.Background()
	if imageID == "" {
		e2e.Logf("there is no image with tag is %s", imageIndex)
		return true, nil
	}
	e2e.Logf("delete image %s\n", imageID)
	_, err = c.CLI.ImageRemove(ctx, imageID, types.ImageRemoveOptions{Force: true})
	if err != nil {
		e2e.Logf("remove docker image %s failed", imageID)
		return false, err
	}
	e2e.Logf("remove image %s success\n", imageID)
	return true, nil
}

// GetImageList is to get the image list
func (c *DockerCLI) GetImageList() ([]string, error) {
	var imageList []string
	ctx := context.Background()

	images, err := c.CLI.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		e2e.Logf("get docker image list failed")
		return imageList, err
	}
	for _, image := range images {
		e2e.Logf("image: %s\n", strings.Join(image.RepoTags, ","))
		imageList = append(imageList, strings.Join(image.RepoTags, ","))
	}
	return imageList, nil
}

// CheckImageExist check the image exist
func (c *DockerCLI) CheckImageExist(imageIndex string) (bool, error) {
	imageList, err := c.GetImageList()
	if err != nil {
		return false, err
	}
	return contains(imageList, imageIndex), nil
}
