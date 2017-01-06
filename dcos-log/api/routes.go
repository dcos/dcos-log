package api

import (
	"context"
	"net/http"

	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/dcos-log/config"
	"github.com/gorilla/mux"
)

type key int

var streamKey key = 1

func requestStreamKeyFromContext(ctx context.Context) bool {
	ctxValue := ctx.Value(streamKey)
	return ctxValue != nil
}

func newAPIRouter(cfg *config.Config, client *http.Client, nodeInfo nodeutil.NodeInfo) (*mux.Router, error) {
	newAuthMiddleware := func(h http.Handler) http.Handler {
		return h
	}

	if cfg.FlagAuth {
		newAuthMiddleware = func(h http.Handler) http.Handler {
			return authMiddleware(h, client, nodeInfo)
		}
	}

	streamMiddleware := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), streamKey, struct{}{})
			h.ServeHTTP(w, req.WithContext(ctx))
		})
	}

	handler := http.HandlerFunc(readJournalHandler)

	r := mux.NewRouter()
	// GET
	logsRange := r.PathPrefix("/range").Subrouter()
	logsRange.Path("/").Handler(handler).Methods("GET")
	logsRange.Path("/framework/{framework_id}/executor/{executor_id}/container/{container_id}").
		Handler(newAuthMiddleware(handler)).Methods("GET")

	// added download headers
	logsRange.Path("/download").Handler(downloadGzippedContentMiddleware(handler, "root-range")).Methods("GET")
	logsRange.Path("/framework/{framework_id}/executor/{executor_id}/container/{container_id}/download").
		Handler(newAuthMiddleware(downloadGzippedContentMiddleware(handler, "task", "container_id"))).Methods("GET")

	logsStream := r.PathPrefix("/stream").Subrouter()
	logsStream.Path("/").Handler(streamMiddleware(handler)).Methods("GET")
	logsStream.Path("/framework/{framework_id}/executor/{executor_id}/container/{container_id}").
		Handler(newAuthMiddleware(streamMiddleware(handler))).Methods("GET")

	// /field/{field} route
	fields := r.PathPrefix("/fields").Subrouter()
	fields.Path("/{field}").HandlerFunc(fieldHandler)

	return r, nil
}
