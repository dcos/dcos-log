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

func MesosFileReader(next http.Handler, client *http.Client, nodeInfo nodeutil.NodeInfo) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		frameworkID := vars["frameworkID"]
		executorID := vars["executorID"]
		containerID := vars["containerID"]

		token, err := GetAuthFromRequest(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Token error: %s", err.Error()), http.StatusUnauthorized)
			return
		}
		logrus.Infof("got token: %s", token)

		header := http.Header{}
		header.Set("Authorization", token)

		logrus.Infof("nodeInfo: %s", nodeInfo)
		mesosID, err := nodeInfo.MesosID(nodeutil.NewContextWithHeaders(context.TODO(), header))
		if err != nil {
			http.Error(w, "Unable to get mesosID: "+err.Error(), http.StatusInternalServerError)
			return
		}

		ip, err := nodeInfo.DetectIP()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		masterURL := url.URL{
			Scheme: "http",
			Host: fmt.Sprintf("%s:%d", ip, 5051),
			Path: "/files/read",
		}

		opts := []reader.Option{reader.OptHeaders(header), reader.OptStream(true),
			reader.OptReadDirection(reader.BottomToTop), reader.OptLines(10)}
		lastEventID := r.Header.Get("Last-Event-ID")
		if lastEventID != "" {
			offset, err := strconv.Atoi(lastEventID)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid Last-Event-ID: %s", err.Error()), http.StatusInternalServerError)
				return
			}

			opts = append(opts, reader.OptOffset(offset))
		}

		reader, err := reader.NewLineReader(client, masterURL, mesosID, frameworkID, executorID, containerID, opts...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logrus.Errorf("unable to initialize files API reader: %s", err)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithFilesAPIContext(r.Context(), reader)))
	})
}
