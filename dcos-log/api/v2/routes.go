package v2

import (
	"net/http"

	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/dcos-log/config"
	"github.com/gorilla/mux"
	"github.com/dcos/dcos-log/dcos-log/api/middleware"
)

// InitRoutes inits the v1 logging routes
func InitRoutes(v2 *mux.Router, cfg *config.Config, client *http.Client, nodeInfo nodeutil.NodeInfo) {
	handler := http.HandlerFunc(readFilesAPI)
	wrapped := middleware.MesosFileReader(handler, cfg, client, nodeInfo)

	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/{file}").Handler(wrapped).Methods("GET")
	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/tasks/{taskPath}/{file}").Handler(wrapped).Methods("GET")

	v2.Path("/task/{taskID}").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		discover(w, req, nodeInfo)
	})
	v2.Path("/task/{taskID}/{file}").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		discover(w, req, nodeInfo)
	})
}
