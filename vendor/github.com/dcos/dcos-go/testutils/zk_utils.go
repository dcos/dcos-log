package testutils

import (
	"fmt"
	"net"
	"runtime"
	"strconv"
	"time"

	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// ZKConfig captures configuration/runtime constraints for a containerized ZK instance.
type ZKConfig struct {
	StartupTimeout time.Duration
	ImageName      string
	Entrypoint     []string
	Command        []string
	ClientPort     int
}

// DefaultZKConfig returns a copy of the default ZK container/runtime configuration.
func DefaultZKConfig() ZKConfig {
	return ZKConfig{
		StartupTimeout: 10 * time.Second,
		ImageName:      "docker.io/jplock/zookeeper:3.4.10",
		Entrypoint:     []string{"/opt/zookeeper/bin/zkServer.sh"},
		Command:        []string{"start-foreground"},
		ClientPort:     2181,
	}
}

// StartZookeeper starts a new zookeeper container.
func StartZookeeper(opts ...func(*ZKConfig)) (*ZkControl, error) {
	config := DefaultZKConfig()
	for _, f := range opts {
		if f != nil {
			f(&config)
		}
	}

	dcli, err := DockerClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not get docker client")
	}

	if err := pullDockerImage(dcli, config.ImageName); err != nil {
		return nil, err
	}

	// the container IP is not routable on Darwin, thus needs port
	// mapping for the container.
	hostConfig := &container.HostConfig{}
	if runtime.GOOS == "darwin" {
		hostConfig.PortBindings = nat.PortMap{
			nat.Port(fmt.Sprintf("%d/tcp", config.ClientPort)): []nat.PortBinding{{
				HostIP:   "0.0.0.0",
				HostPort: strconv.Itoa(config.ClientPort),
			}},
		}
	}

	r, err := dcli.ContainerCreate(
		context.Background(),
		&container.Config{
			Image:      config.ImageName,
			Entrypoint: config.Entrypoint,
			Cmd:        config.Command,
		},
		hostConfig,
		nil, "")
	if err != nil {
		return nil, errors.Wrap(err, "could not create zk container")
	}

	// create a teardown that will be used here to try to tear down the
	// container if anything fails in setup
	cleanup := func() {
		removeContainer(dcli, r.ID)
	}

	// start the container
	if err := dcli.ContainerStart(context.Background(), r.ID, types.ContainerStartOptions{}); err != nil {
		cleanup()
		return nil, errors.Wrap(err, "could not start zk container")
	}

	info, err := dcli.ContainerInspect(context.Background(), r.ID)
	if err != nil {
		cleanup()
		return nil, errors.Wrap(err, "could not inspect container")
	}

	var addr string
	if runtime.GOOS == "darwin" {
		addr = "127.0.0.1:" + strconv.Itoa(config.ClientPort)
	} else {
		addr = net.JoinHostPort(info.NetworkSettings.IPAddress, strconv.Itoa(config.ClientPort))
	}

	done := make(chan struct{})
	defer close(done)

	connected := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					time.Sleep(1)
					continue
				}
				fmt.Println("successfully connected to ZK at", addr)
				conn.Close()
				close(connected)
				return
			}

		}
	}()
	select {
	case <-connected:
	case <-time.After(config.StartupTimeout):
		cleanup()
		return nil, errors.Errorf("could not connect to zookeeper in %s", config.StartupTimeout)
	}
	control := &ZkControl{
		dockerClient: dcli,
		containerID:  r.ID,
		addr:         addr,
	}
	return control, nil
}
