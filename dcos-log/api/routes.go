package api

import (
	"net/http"
	"net/http/pprof"

	"github.com/dcos/dcos-log/dcos-log/config"
	"github.com/dcos/dcos-log/dcos-log/router"
	"github.com/gorilla/mux"
)

func loadRoutes(cfg *config.Config) []router.Route {
	routes := logRoutes()

	if cfg.FlagDebug {
		routes = append(routes, debugRoutes()...)
	}

	return routes
}

func logRoutes() []router.Route {
	return []router.Route{
		// wait for the new logs, server will not close the connection
		{
			URL:     "/stream",
			Handler: streamingServerSSEHandler,
			Headers: []string{"Accept", "text/event-stream"},
		},
		{
			URL:     "/stream",
			Handler: streamingServerJSONHandler,
			Headers: []string{"Accept", "application/json"},
		},
		{
			URL:     "/stream",
			Handler: streamingServerTextHandler,
		},

		// get a range of logs, do not wait
		{
			URL:     "/logs",
			Handler: rangeServerSSEHandler,
			Headers: []string{"Accept", "text/event-stream"},
		},
		{
			URL:     "/logs",
			Handler: rangeServerJSONHandler,
			Headers: []string{"Accept", "application/json"},
		},
		{
			URL:     "/logs",
			Handler: rangeServerTextHandler,
		},
	}
}

func debugRoutes() []router.Route {
	return []router.Route{
		{
			URL:     "/debug/pprof/",
			Handler: pprof.Index,
			Gzip:    true,
		},
		{
			URL:     "/debug/pprof/cmdline",
			Handler: pprof.Cmdline,
			Gzip:    true,
		},
		{
			URL:     "/debug/pprof/profile",
			Handler: pprof.Profile,
			Gzip:    true,
		},
		{
			URL:     "/debug/pprof/symbol",
			Handler: pprof.Symbol,
			Gzip:    true,
		},
		{
			URL:     "/debug/pprof/trace",
			Handler: pprof.Trace,
			Gzip:    true,
		},
		{
			URL: "/debug/pprof/{profile}",
			Handler: func(w http.ResponseWriter, req *http.Request) {
				profile := mux.Vars(req)["profile"]
				pprof.Handler(profile).ServeHTTP(w, req)
			},
			Gzip: true,
		},
	}
}
