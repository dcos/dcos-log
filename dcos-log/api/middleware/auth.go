package middleware

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dcos/dcos-go/dcos"
	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/gorilla/mux"
)

const (
	sandboxURLScheme  = "https"
	sandboxPath       = "/files/browse"
	sandboxBrowsePath = "/var/lib/mesos/slave/slaves"
	sandboxFrameworks = "frameworks"
	sandboxExecutors  = "executors"
	sandboxRuns       = "runs"
)

// ErrMissingToken is returned by GetAuthFromRequest when JWT is missing.
var ErrMissingToken = errors.New("Missing token in auth request")

func getSandboxURL(nodeInfo nodeutil.NodeInfo, role string) (*url.URL, error) {
	mesosPort := dcos.PortMesosAgent
	if role == dcos.RoleMaster {
		mesosPort = dcos.PortMesosMaster
	}

	detectedIP, err := nodeInfo.DetectIP()
	if err != nil {
		return nil, err
	}

	// prepare sandbox URL
	sandboxBaseURL := &url.URL{
		Scheme: sandboxURLScheme,
		Host:   net.JoinHostPort(detectedIP.String(), strconv.Itoa(mesosPort)),
		Path:   sandboxPath,
	}

	return sandboxBaseURL, nil
}

// validate the token
func validateToken(t string) (string, error) {
	if !strings.HasPrefix(t, "token=") {
		return t, ErrMissingToken
	}

	return t, nil
}

// GetAuthFromRequest will try to extract JWT from Authorization header.
func GetAuthFromRequest(r *http.Request) (string, error) {
	// give priority to Authorization header
	authorizationHeader := r.Header.Get("Authorization")
	if authorizationHeader != "" {
		return validateToken(authorizationHeader)
	}

	return "", ErrMissingToken
}

// AuthMiddleware is a middleware that validates a user has a valid JWT to access the given endpoint.
func AuthMiddleware(next http.Handler, client *http.Client, nodeInfo nodeutil.NodeInfo, role string) http.Handler {
	if nodeInfo == nil {
		panic("nodeInfo cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// JWT is required to present in a request. The middleware will extract the token and try to access
		// sandbox with it. We authorize the request if sandbox returns 200.
		token, err := GetAuthFromRequest(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Token error: %s", err.Error()), http.StatusUnauthorized)
			return
		}

		// frameworkID, executorID and containerID are required mux variables to authorize a request.
		muxVars := mux.Vars(r)
		frameworkID := muxVars["framework_id"]
		executorID := muxVars["executor_id"]
		containerID := muxVars["container_id"]

		// if we ended up in this handler without required mux variables, we are doing something wrong.
		if frameworkID == "" || executorID == "" || containerID == "" {
			http.Error(w, "Missing mux variables `frameworkID`, `executorID` or `containerID`", http.StatusBadRequest)
			return
		}

		sandboxBaseURL, err := getSandboxURL(nodeInfo, role)
		if err != nil {
			http.Error(w, "Unable to get sandboxBaseURL: "+err.Error(), http.StatusInternalServerError)
			return
		}

		header := http.Header{}
		header.Set("Authorization", token)

		mesosID, err := nodeInfo.MesosID(nodeutil.NewContextWithHeaders(nil, header))
		if err != nil {
			http.Error(w, "Unable to get mesosID: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// "/var/lib/mesos/slave/slaves/<mesos_id>/frameworks/<framework_id>/executors/<executor_id>/runs/<container_id>"
		sandboxPath := filepath.Join(sandboxBrowsePath, mesosID, sandboxFrameworks, frameworkID, sandboxExecutors,
			executorID, sandboxRuns, containerID)
		sandboxBaseURL.RawQuery = "path=" + url.QueryEscape(sandboxPath)

		req, err := http.NewRequest("GET", sandboxBaseURL.String(), nil)
		if err != nil {
			http.Error(w, "Invalid request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		req.Header.Add("Authorization", token)

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Could not make auth request: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// get a response code and close response body before serving the request.
		responseCode := resp.StatusCode
		resp.Body.Close()

		if responseCode != http.StatusOK {
			http.Error(w, fmt.Sprintf("Auth URL %s. Invalid auth response code: %d", sandboxBaseURL.String(), responseCode), responseCode)
			return
		}
		next.ServeHTTP(w, r)
	})
}
