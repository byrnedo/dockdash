package dockerClient

import (
	docker "github.com/fsouza/go-dockerclient"
	"strings"
)

type DockerClient struct {
	*docker.Client
}

func NewDockerClient() (c *DockerClient, err error) {
	c = &DockerClient{}
	c.Client, err = docker.NewClientFromEnv()
	if err != nil {
		return
	}
	return

}

func (dc *DockerClient) GetDockerPs() []string {
	containers, err := dc.ListContainers(docker.ListContainersOptions{All: false})
	if err != nil {
		return []string{"Error:" + err.Error()}
	}

	list := make([]string, len(containers))
	for i, container := range containers {
		name := strings.Join(container.Names, " ")
		list[i] = name + "\t" + container.Image
	}
	return list

}
