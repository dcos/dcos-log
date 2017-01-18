package api

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/activation"
	"github.com/dcos/dcos-go/dcos"
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

// loadCAPool will load a valid x509 cert.
func loadCAPool(path string) (*x509.CertPool, error) {
	caPool := x509.NewCertPool()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if !caPool.AppendCertsFromPEM(b) {
		return nil, errors.New("CACertFile parsing failed")
	}

	return caPool, nil
}

// StartServer is an entry point to dcos-log service.
func StartServer(cfg *config.Config) error {
	tr := &http.Transport{
		DisableKeepAlives: true,
	}

	// if user provided CA cert we must use it, otherwise use InsecureSkipVerify: true for all HTTPS requests.
	if cfg.FlagCACertFile != "" {
		logrus.Infof("Loading CA cert: %s", cfg.FlagCACertFile)
		caPool, err := loadCAPool(cfg.FlagCACertFile)
		if err != nil {
			return err
		}

		tr.TLSClientConfig = &tls.Config{
			RootCAs: caPool,
		}
	} else {
		tr.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
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
