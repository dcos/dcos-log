package testutils

import (
	"os"
	"strings"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// DockerClient returns a new docker client
func DockerClient() (*client.Client, error) {
	dockerCli, err := client.NewEnvClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not connect to docker")
	}
	return dockerCli, nil
}

func removeContainer(dcli *client.Client, ctrIDs ...string) error {
	var errs []string
	for _, ctrID := range ctrIDs {
		if err := dcli.ContainerRemove(context.Background(), ctrID,
			types.ContainerRemoveOptions{
				RemoveVolumes: true,
				Force:         true,
			}); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ","))
	}

	return nil
}

func pullDockerImage(dcli *client.Client, image string) error {
	exists, err := imageExists(dcli, image)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	resp, err := dcli.ImagePull(context.Background(), image, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	fd, isTerm := term.GetFdInfo(os.Stdout)

	return jsonmessage.DisplayJSONMessagesStream(resp, os.Stdout, fd, isTerm, nil)
}

func imageExists(dcli *client.Client, image string) (bool, error) {
	_, _, err := dcli.ImageInspectWithRaw(context.Background(), image)
	if err == nil {
		return true, nil
	}

	if client.IsErrImageNotFound(err) {
		return false, nil
	}

	return false, err
}
