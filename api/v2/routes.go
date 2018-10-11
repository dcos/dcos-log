package v2

import (
	"net/http"
	"path"

	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/api/middleware"
	"github.com/dcos/dcos-log/config"
	"github.com/gorilla/mux"
)

const (
	taskPath       = "/task/frameworks/{frameworkID}/executors/{executorID}/runs/{containerID}"
	taskBrowsePath = taskPath + "/files/browse"
	podPath        = taskPath + "/tasks/{taskPath}"
	podBrowsePath  = podPath + "/files/browse"
	discoverPath   = "/task/{taskID}"
	componentPath  = "/component"
)

// InitRoutes inits the v1 logging routes
func InitRoutes(v2 *mux.Router, cfg *config.Config, client *http.Client, nodeInfo nodeutil.NodeInfo) {
	// browse sandbox files
	wrappedBrowseFiles := middleware.Wrapped(http.HandlerFunc(browseFiles), cfg, client, nodeInfo)
	v2.Path(taskBrowsePath).Handler(wrappedBrowseFiles).Methods("GET")
	v2.Path(podBrowsePath).Handler(wrappedBrowseFiles).Methods("GET")

	// task logs
	wrappedTaskLogHandler := middleware.Wrapped(http.HandlerFunc(filesAPIHandler), cfg, client, nodeInfo)
	v2.Path(path.Join(taskPath, "/{file}")).Handler(wrappedTaskLogHandler).Methods("GET")
	v2.Path(podPath + "/{file}").Handler(wrappedTaskLogHandler).Methods("GET")

	// discover endpoints
	wrappedDiscoverHandler := middleware.Wrapped(http.HandlerFunc(discoverHandler), cfg, client, nodeInfo)
	wrappedDiscoverBrowseHandler := middleware.Wrapped(http.HandlerFunc(browseHandler), cfg, client, nodeInfo)
	wrappedDiscoverDownloadHandler := middleware.Wrapped(http.HandlerFunc(downloadHandler), cfg, client, nodeInfo)

	v2.Path(discoverPath).Handler(wrappedDiscoverHandler).Methods("GET")
	v2.Path(path.Join(discoverPath, "/file/{file}")).Handler(wrappedDiscoverHandler).Methods("GET")

	// browse files
	v2.Path(path.Join(discoverPath, "/browse")).Handler(wrappedDiscoverBrowseHandler).Methods("GET")

	// download a file, default to stdout
	v2.Path(path.Join(discoverPath, "/download")).Handler(wrappedDiscoverDownloadHandler).Methods("GET")
	v2.Path(path.Join(discoverPath, "/file/{file}/download")).Handler(wrappedDiscoverDownloadHandler).Methods("GET")

	// component logs
	wrappedComponentHandler := middleware.Wrapped(http.HandlerFunc(journalHandler), cfg, client, nodeInfo)
	v2.Path(componentPath).Handler(wrappedComponentHandler).Methods("GET")
	v2.Path(path.Join(componentPath, "/{name}")).Handler(wrappedComponentHandler).Methods("GET")

	// download path
	wrappedDownloadHandler := middleware.Wrapped(http.HandlerFunc(downloadFile), cfg, client, nodeInfo)
	v2.Path(path.Join(taskPath, "/{file}/download")).Handler(wrappedDownloadHandler).Methods("GET")
	v2.Path(path.Join(podPath, "/{file}/download")).Handler(wrappedDownloadHandler).Methods("GET")
}
