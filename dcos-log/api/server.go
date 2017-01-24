package api

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/activation"
	"github.com/dcos/dcos-go/dcos"
	"github.com/dcos/dcos-go/dcos/http/transport"
	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/dcos-log/config"
)

// override the defaultStateURL to use https scheme
var defaultStateURL = url.URL{
	Scheme: "https",
	Host:   net.JoinHostPort(dcos.DNSRecordLeader, strconv.Itoa(dcos.PortMesosMaster)),
	Path:   "/state",
}

func newNodeInfo(cfg *config.Config, client *http.Client) (nodeutil.NodeInfo, error) {
	if !cfg.FlagAuth {
		return nil, nil
	}

	// if auth is enabled we will also make requests to mesos via https.
	nodeInfo, err := nodeutil.NewNodeInfo(client, cfg.FlagRole, nodeutil.OptionMesosStateURL(defaultStateURL.String()))
	if err != nil {
		return nil, err
	}

	return nodeInfo, nil
}

// StartServer is an entry point to dcos-log service.
func StartServer(cfg *config.Config) error {
	transportOptions := []transport.OptionTransportFunc{}
	if cfg.FlagCACertFile != "" {
		transportOptions = append(transportOptions, transport.OptionIAMConfigPath(cfg.FlagCACertFile))
	}

	tr, err := transport.NewTransport(transportOptions...)
	if err != nil {
		return err
	}

	// update get request timeout.
	timeout, err := time.ParseDuration(cfg.FlagGetRequestTimeout)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}

	// pass a copy of client because newNodeInfo may modify Transport.
	nodeInfo, err := newNodeInfo(cfg, client)
	if err != nil {
		return err
	}

	router, err := newAPIRouter(cfg, client, nodeInfo)
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
