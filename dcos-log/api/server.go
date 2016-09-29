package api

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/dcos/dcos-log/dcos-log/config"
	"github.com/dcos/dcos-log/dcos-log/router"
)

// StartServer is an entry point to dcos-log service.
func StartServer(cfg *config.Config) error {
	router, err := router.NewRouter(loadRoutes())
	if err != nil {
		return err
	}

	logrus.Infof("Starting web server on %d", cfg.FlagPort)
	return http.ListenAndServe(fmt.Sprintf(":%d", cfg.FlagPort), router)
}
