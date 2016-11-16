package api

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-systemd/activation"
	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-go/jwt/transport"
	"github.com/dcos/dcos-log/dcos-log/config"
)

func newNodeInfo(cfg *config.Config, clientWithJWT http.Client) (nodeutil.NodeInfo, error) {
	if !cfg.FlagAuth {
		return nil, nil
	}

	rt, err := transport.NewRoundTripper(clientWithJWT.Transport, transport.OptionReadIAMConfig(cfg.FlagIAMConfig))
	if err != nil {
		return nil, fmt.Errorf("Unable to create secure JWT transport. -iam-config must be used with -auth: %s", err)
	}

	clientWithJWT.Transport = rt

	nodeInfo, err := nodeutil.NewNodeInfo(&clientWithJWT)
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
	nodeInfo, err := newNodeInfo(cfg, *client)
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
