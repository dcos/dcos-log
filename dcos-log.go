package main

import (
	"os"

	"github.com/dcos/dcos-log/api"
	"github.com/dcos/dcos-log/config"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg, err := config.NewConfig(os.Args)
	if err != nil {
		logrus.Fatalf("Could not load config: %s", err)
	}

	logrus.Fatal(api.StartServer(cfg))
}
