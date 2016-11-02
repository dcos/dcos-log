package api

import (
	"net/http"
	"net/http/pprof"

	"github.com/dcos/dcos-log/dcos-log/config"
	"github.com/dcos/dcos-log/dcos-log/journal/reader"
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

func decorateHandler(stream bool, formatter reader.EntryFormatter) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		readJournalHandler(w, req, stream, formatter)
	}
}

func dispatchLogs(url string, stream bool) []router.Route {
	var routes []router.Route
	for _, r := range []struct {
		headers   []string
		formatter reader.EntryFormatter
	}{
		{
			formatter: reader.FormatText{},
			headers:   []string{"Accept", "text/(plain|html)"},
		},
		{
			formatter: reader.FormatText{},
			headers:   []string{"Accept", "\\*/\\*"},
		},
		{
			formatter: reader.FormatJSON{},
			headers:   []string{"Accept", "application/json"},
		},
		{
			formatter: reader.FormatSSE{
				UseCursorID: true,
			},
			headers: []string{"Accept", "text/event-stream"},
		},
	} {
		routes = append(routes, router.Route{
			URL:     url,
			Handler: decorateHandler(stream, r.formatter),
			Headers: r.headers,
		})
	}
	return routes
}

func logRoutes() []router.Route {
	logsRange := dispatchLogs("/logs", false)
	containerLogsRange := dispatchLogs("/logs/container/{container_id}", false)
	logsStream := dispatchLogs("/stream", true)
	containerLogsStream := dispatchLogs("/stream/container/{container_id}", true)
	extraRoutes := []router.Route{
		{
			URL:     "/fields/{field}",
			Handler: fieldHandler,
		},
	}

	// build all routes
	var routes []router.Route
	for _, r := range [][]router.Route{logsRange, containerLogsRange, logsStream, containerLogsStream, extraRoutes} {
		routes = append(routes, r...)
	}
	return routes
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
