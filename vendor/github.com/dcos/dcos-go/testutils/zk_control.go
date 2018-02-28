package testutils

import (
	"log"
	"sync"

	"github.com/docker/engine-api/client"
	"github.com/pkg/errors"
)

// ZkControl allows testing code to manipulate a running ZK instance.
type ZkControl struct {
	dockerClient *client.Client
	containerID  string
	addr         string
	teardownOnce sync.Once
}

// Addr returns the address of the zookeeper node
func (z *ZkControl) Addr() string {
	return z.addr
}

// Teardown destroys the ZK container
func (z *ZkControl) Teardown() error {
	log.Println("Starting requested teardown of ZK container")
	var err error
	z.teardownOnce.Do(func() {
		err = removeContainer(z.dockerClient, z.containerID)
		if err == nil {
			log.Println("Successfully removed ZK container")
		}
	})
	if err != nil {
		return errors.Wrap(err, "could not remove ZK container")
	}
	return nil
}

// TeardownPanic destroys the ZK container and panics if unsuccessful
func (z *ZkControl) TeardownPanic() {
	if err := z.Teardown(); err != nil {
		panic(err)
	}
}
