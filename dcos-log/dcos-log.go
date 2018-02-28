package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/dcos/dcos-log/dcos-log/api"
	"github.com/dcos/dcos-log/dcos-log/config"
)

func main() {
	cfg, err := config.NewConfig(os.Args)
	if err != nil {
		logrus.Fatalf("Could not load config: %s", err)
	}

	logrus.Fatal(api.StartServer(cfg))
}
