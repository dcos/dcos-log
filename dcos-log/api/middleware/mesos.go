package middleware

import (
	"context"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/dcos-log/config"
)

type key int

var (
	cfgKey        key
	httpClientKey key = 1
	nodeInfoKey   key = 2
	tokenKey      key = 3
)

// withKeyContext returns a context with an encapsulated object by a key.
func withKeyContext(ctx context.Context, k key, obj interface{}) context.Context {
	return context.WithValue(ctx, k, obj)
}

// fromKeyContext returns an object from a context by a key.
func fromContextByKey(ctx context.Context, k key) (interface{}, bool) {
	instance := ctx.Value(k)
	return instance, instance != nil
}

// WithConfigContext wraps a config object into context.
func WithConfigContext(ctx context.Context, cfg *config.Config) context.Context {
	return withKeyContext(ctx, cfgKey, cfg)
}

// FromContextConfig returns a config object from a context
func FromContextConfig(ctx context.Context) (cfg *config.Config, ok bool) {
	instance, ok := fromContextByKey(ctx, cfgKey)
	if !ok {
		return nil, ok
	}

	cfg, ok = instance.(*config.Config)
	return cfg, ok
}

// WithHTTPClientContext wraps a *http.Client object into context.
func WithHTTPClientContext(ctx context.Context, client *http.Client) context.Context {
	return withKeyContext(ctx, httpClientKey, client)
}

// FromContextHTTPClient returns an *http.Client object from a context
func FromContextHTTPClient(ctx context.Context) (client *http.Client, ok bool) {
	instance, ok := fromContextByKey(ctx, httpClientKey)
	if !ok {
		return nil, ok
	}

	client, ok = instance.(*http.Client)
	return client, ok
}

// WithNodeInfoContext wraps the NodeInfo object into context.
func WithNodeInfoContext(ctx context.Context, nodeInfo nodeutil.NodeInfo) context.Context {
	return withKeyContext(ctx, nodeInfoKey, nodeInfo)
}

// FromContextNodeInfo returns a nodeInfo object from a context.
func FromContextNodeInfo(ctx context.Context) (nodeInfo nodeutil.NodeInfo, ok bool) {
	instance, ok := fromContextByKey(ctx, nodeInfoKey)
	if !ok {
		return nil, ok
	}

	nodeInfo, ok = instance.(nodeutil.NodeInfo)
	return nodeInfo, ok
}

// FromContextToken returns a token string from a context if available.
func FromContextToken(ctx context.Context) (string, bool) {
	instance, ok := fromContextByKey(ctx, tokenKey)
	if !ok {
		return "", ok
	}

	token, ok := instance.(*string)
	return *token, ok
}

// Wrapped wraps an http handler with values in a context.
func Wrapped(next http.Handler, cfg *config.Config, client *http.Client, nodeInfo nodeutil.NodeInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = WithConfigContext(ctx, cfg)
		ctx = WithHTTPClientContext(ctx, client)
		ctx = WithNodeInfoContext(ctx, nodeInfo)

		// wrap the token string is available
		token, err := GetAuthFromRequest(r)
		if err == nil && token != "" {
			ctx = withKeyContext(ctx, tokenKey, &token)
		} else {
			logrus.Warnf("Authorization token not found: %s", err)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
