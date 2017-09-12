package v2

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/dcos-log/config"
)

// InitRoutes inits the v1 logging routes
func InitRoutes(v2 *mux.Router, cfg *config.Config, client *http.Client, nodeInfo nodeutil.NodeInfo) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		readFilesAPI(w, req, client)
	})
	v2.Path("/stream/{frameworkID}/{executorID}/{containerID}").Handler(handler).Methods("GET")
	v2.Path("/{taskID}").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		discover(w, req, client)
	})
}
