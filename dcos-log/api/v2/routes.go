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
	// browse sandbox files
	wrappedBrowseFiles := middleware.Wrapped(http.HandlerFunc(browseFiles), cfg, client, nodeInfo)
	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/files/browse").Handler(wrappedBrowseFiles).Methods("GET")
	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/tasks/{taskPath}/files/browse").Handler(wrappedBrowseFiles).Methods("GET")

	// task logs
	wrappedTaskLogHandler := middleware.Wrapped(http.HandlerFunc(filesAPIHandler), cfg, client, nodeInfo)
	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/{file}").Handler(wrappedTaskLogHandler).Methods("GET")
	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/tasks/{taskPath}/{file}").Handler(wrappedTaskLogHandler).Methods("GET")

	// discover endpoints
	wrappedDiscoverHandler := middleware.Wrapped(http.HandlerFunc(discoverHandler), cfg, client, nodeInfo)
	wrappedDiscoverBrowseHandler := middleware.Wrapped(http.HandlerFunc(browseHandler), cfg, client, nodeInfo)
	wrappedDiscoverDownloadHandler := middleware.Wrapped(http.HandlerFunc(downloadHandler), cfg, client, nodeInfo)

	v2.Path("/task/{taskID}").Handler(wrappedDiscoverHandler).Methods("GET")
	v2.Path("/task/{taskID}/file/{file}").Handler(wrappedDiscoverHandler).Methods("GET")

	// browse files
	v2.Path("/task/{taskID}/browse").Handler(wrappedDiscoverBrowseHandler).Methods("GET")

	// download a file, default to stdout
	v2.Path("/task/{taskID}/download").Handler(wrappedDiscoverDownloadHandler).Methods("GET")
	v2.Path("/task/{taskID}/file/{file}/download").Handler(wrappedDiscoverDownloadHandler).Methods("GET")

	// component logs
	wrappedComponentHandler := middleware.Wrapped(http.HandlerFunc(journalHandler), cfg, client, nodeInfo)
	v2.Path("/component").Handler(wrappedComponentHandler).Methods("GET")
	v2.Path("/component/{name}").Handler(wrappedComponentHandler).Methods("GET")

	// download path
	wrappedDownloadHandler := middleware.Wrapped(http.HandlerFunc(downloadFile), cfg, client, nodeInfo)
	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/{file}/download").Handler(wrappedDownloadHandler).Methods("GET")
	v2.Path("/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}/tasks/{taskPath}/{file}/download").Handler(wrappedDownloadHandler).Methods("GET")
}
