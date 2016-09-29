package api

import "github.com/dcos/dcos-log/dcos-log/router"

func loadRoutes() []router.Route {
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
			Headers: []string{"Accept", "text/(plain|html)"},
		},
		{
			URL:     "/stream",
			Handler: streamingServerTextHandler,
			Headers: []string{"Accept", "\\*/\\*"},
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
			Headers: []string{"Accept", "text/(plain|html)"},
		},
		{
			URL:     "/logs",
			Handler: rangeServerTextHandler,
			Headers: []string{"Accept", "\\*/\\*"},
		},
	}
}
