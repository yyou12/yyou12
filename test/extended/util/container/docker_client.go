package container

import (
	"context"
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
	CLI *client.Client
}

// NewDockerCLI initialize the docker cli framework
func NewDockerCLI() *DockerCLI {
	newclient := &DockerCLI{}
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		e2e.Failf("get docker client failed")
	}
	newclient.CLI = cli
	return newclient
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
