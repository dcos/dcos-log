package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/dcos/dcos-go/dcos-log/api"
	"github.com/dcos/dcos-go/dcos-log/config"
)

func main() {
	cfg, err := config.NewConfig(os.Args)
	if err != nil {
		logrus.Fatalf("Could not load config: %s", err)
	}

	logrus.Fatal(api.StartServer(cfg))
}
