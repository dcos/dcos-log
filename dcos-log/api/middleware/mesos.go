package middleware

import (
	"net/http"
	"github.com/gorilla/mux"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/dcos/dcos-go/dcos/nodeutil"
	"context"
	"net/url"
	"github.com/dcos/dcos-log/dcos-log/mesos/files/reader"
	"strconv"
	"io"
	"github.com/dcos/dcos-log/dcos-log/config"
	"time"
	"net"
	"github.com/dcos/dcos-go/dcos"
)

type key int

var mesosFilesAPIKey = 1

func WithFilesAPIContext(ctx context.Context, r io.Reader) context.Context {
	return context.WithValue(ctx, mesosFilesAPIKey, r)
}

func FromContext(ctx context.Context) (io.Reader, error) {
	instance := ctx.Value(mesosFilesAPIKey)
	if instance == nil {
		return nil, fmt.Errorf("context does not hold mesosFIlesAPIKey")
	}

	reader, ok := instance.(io.Reader)
	if !ok {
		return nil, fmt.Errorf("context does not hold an instance of mesos files API reader")
	}

	return reader, nil
}

func MesosFileReader(next http.Handler, cfg *config.Config, client *http.Client, nodeInfo nodeutil.NodeInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme := "http"
		if cfg.FlagAuth {
			scheme = "https"
		}

		vars := mux.Vars(r)
		frameworkID := vars["frameworkID"]
		executorID := vars["executorID"]
		containerID := vars["containerID"]
		taskPath := vars["taskPath"]
		file := vars["file"]

		token, err := GetAuthFromRequest(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Token error: %s", err.Error()), http.StatusUnauthorized)
			return
		}

		header := http.Header{}
		header.Set("Authorization", token)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		mesosID, err := nodeInfo.MesosID(nodeutil.NewContextWithHeaders(ctx, header))
		if err != nil {
			http.Error(w, "unable to get mesosID: "+err.Error(), http.StatusInternalServerError)
			return
		}

		ip, err := nodeInfo.DetectIP()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		masterURL := url.URL{
			Scheme: scheme,
			Host: net.JoinHostPort(ip.String(), strconv.Itoa(dcos.PortMesosAgent)),
			Path: "/files/read",
		}

		opts := []reader.Option{reader.OptHeaders(header), reader.OptStream(true)}
		lastEventID := r.Header.Get("Last-Event-ID")
		if lastEventID != "" {
			offset, err := strconv.Atoi(lastEventID)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid Last-Event-ID: %s", err.Error()), http.StatusInternalServerError)
				return
			}

			opts = append(opts, reader.OptOffset(offset))
		}

		defaultFormatter := reader.LineFormat
		if r.Header.Get("Accept") == "text/event-stream" {
			defaultFormatter = reader.SSEFormat
		}

		reader, err := reader.NewLineReader(client, masterURL, mesosID, frameworkID, executorID, containerID,
			taskPath, file, defaultFormatter, opts...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logrus.Errorf("unable to initialize files API reader: %s", err)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithFilesAPIContext(r.Context(), reader)))
	})
}
