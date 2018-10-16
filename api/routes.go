package api

import (
	"net/http"

	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/api/v1"
	"github.com/dcos/dcos-log/api/v2"
	"github.com/dcos/dcos-log/config"
	"github.com/gorilla/mux"
)

func newAPIRouter(cfg *config.Config, client *http.Client, nodeInfo nodeutil.NodeInfo) (*mux.Router, error) {
	r := mux.NewRouter()

	// define top level subrouter for base endpoint /v1
	v1Subrouter := r.PathPrefix("/v1").Subrouter()
	v1.InitRoutes(v1Subrouter, cfg, client, nodeInfo)

	v2Subrouter := r.PathPrefix("/v2").Subrouter()
	v2.InitRoutes(v2Subrouter, cfg, client, nodeInfo)

	return r, nil
}
