package api

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/activation"
	"github.com/dcos/dcos-log/dcos-log/config"
	"github.com/dcos/dcos-log/dcos-log/router"
)

// StartServer is an entry point to dcos-log service.
func StartServer(cfg *config.Config) error {
	router, err := router.NewRouter(loadRoutes(cfg))
	if err != nil {
		return err
	}

	listeners, err := activation.Listeners(true)
	if err != nil {
		return fmt.Errorf("Unable to get listeners: %s", err)
	}

	// Listen on unix socket
	if len(listeners) == 1 {
		logrus.Infof("Listen on %s", listeners[0].Addr().String())
		return http.Serve(listeners[0], router)
	}

	logrus.Infof("Starting web server on %d", cfg.FlagPort)
	return http.ListenAndServe(fmt.Sprintf(":%d", cfg.FlagPort), router)
}
