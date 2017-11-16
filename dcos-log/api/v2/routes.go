package v2

import (
	"net/http"

	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/dcos-log/api/middleware"
	"github.com/dcos/dcos-log/dcos-log/config"
	"github.com/gorilla/mux"
)

// InitRoutes inits the v1 logging routes
func InitRoutes(v2 *mux.Router, cfg *config.Config, client *http.Client, nodeInfo nodeutil.NodeInfo) {
	taskLogHandler := http.HandlerFunc(filesAPIHandler)
	wrappedTaskLogHandler := middleware.Wrapped(taskLogHandler, cfg, client, nodeInfo)

	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/{file}").Handler(wrappedTaskLogHandler).Methods("GET")
	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/tasks/{taskPath}/{file}").Handler(wrappedTaskLogHandler).Methods("GET")

	discoverHandler := http.HandlerFunc(discover)
	wrappedDiscoverHandler := middleware.Wrapped(discoverHandler, cfg, client, nodeInfo)

	v2.Path("/task/{taskID}").Handler(wrappedDiscoverHandler).Methods("GET")
	v2.Path("/task/{taskID}/file/{file}").Handler(wrappedDiscoverHandler).Methods("GET")
}
