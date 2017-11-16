package v1

import (
	"context"
	"net/http"

	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/dcos-log/api/middleware"
	"github.com/dcos/dcos-log/dcos-log/config"
	"github.com/gorilla/mux"
)

type key int

var streamKey key = 1

func requestStreamKeyFromContext(ctx context.Context) bool {
	ctxValue := ctx.Value(streamKey)
	return ctxValue != nil
}

// InitRoutes inits the v1 logging routes
func InitRoutes(v1 *mux.Router, cfg *config.Config, client *http.Client, nodeInfo nodeutil.NodeInfo) {
	newAuthMiddleware := func(h http.Handler) http.Handler {
		return h
	}

	if cfg.FlagAuth {
		newAuthMiddleware = func(h http.Handler) http.Handler {
			return middleware.AuthMiddleware(h, client, nodeInfo, cfg.FlagRole)
		}
	}

	streamMiddleware := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), streamKey, struct{}{})
			h.ServeHTTP(w, req.WithContext(ctx))
		})
	}

	handler := http.HandlerFunc(readJournalHandler)

	v1.Path("/range/").Handler(handler).Methods("GET")
	v1.Path("/range/framework/{framework_id}/executor/{executor_id}/container/{container_id}").
		Handler(newAuthMiddleware(handler)).Methods("GET")

	v1.Path("/range/download").Handler(middleware.DownloadGzippedContent(handler, "root-range")).Methods("GET")
	v1.Path("/range/framework/{framework_id}/executor/{executor_id}/container/{container_id}/download").
		Handler(newAuthMiddleware(middleware.DownloadGzippedContent(handler, "task", "container_id"))).Methods("GET")

	v1.Path("/stream/").Handler(streamMiddleware(handler)).Methods("GET")
	v1.Path("/stream/framework/{framework_id}/executor/{executor_id}/container/{container_id}").
		Handler(newAuthMiddleware(streamMiddleware(handler))).Methods("GET")

	v1.Path("/fields/{field}").HandlerFunc(fieldHandler)
}
