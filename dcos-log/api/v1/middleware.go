package v1

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
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
	// authCookieName is a token name passed by a browser in `Cookie` request header.
	authCookieName = "dcos-acs-auth-cookie"

	sandboxURLScheme  = "https"
	sandboxPath       = "/files/browse"
	sandboxBrowsePath = "/var/lib/mesos/slave/slaves"
	sandboxFrameworks = "frameworks"
	sandboxExecutors  = "executors"
	sandboxRuns       = "runs"
)

// ErrMissingToken is returned by getAuthFromRequest when JWT is missing.
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

// getAuthFromRequest will try to extract JWT from Authorization header.
func getAuthFromRequest(r *http.Request) (string, error) {
	// give priority to Authorization header
	authorizationHeader := r.Header.Get("Authorization")
	if authorizationHeader != "" {
		return validateToken(authorizationHeader)
	}

	return "", ErrMissingToken
}

func authMiddleware(next http.Handler, client *http.Client, nodeInfo nodeutil.NodeInfo, role string) http.Handler {
	if nodeInfo == nil {
		panic("nodeInfo cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// JWT is required to present in a request. The middleware will extract the token and try to access
		// sandbox with it. We authorize the request if sandbox returns 200.
		token, err := getAuthFromRequest(r)
		if err != nil {
			httpError(w, fmt.Sprintf("Token error: %s", err.Error()), http.StatusUnauthorized, r)
			return
		}

		// frameworkID, executorID and containerID are required mux variables to authorize a request.
		muxVars := mux.Vars(r)
		frameworkID := muxVars["framework_id"]
		executorID := muxVars["executor_id"]
		containerID := muxVars["container_id"]

		// if we ended up in this handler without required mux variables, we are doing something wrong.
		if frameworkID == "" || executorID == "" || containerID == "" {
			httpError(w, "Missing mux variables `frameworkID`, `executorID` or `containerID`", http.StatusBadRequest, r)
			return
		}

		sandboxBaseURL, err := getSandboxURL(nodeInfo, role)
		if err != nil {
			httpError(w, "Unable to get sandboxBaseURL: "+err.Error(), http.StatusInternalServerError, r)
			return
		}

		header := http.Header{}
		header.Set("Authorization", token)

		mesosID, err := nodeInfo.MesosID(nodeutil.NewContextWithHeaders(nil, header))
		if err != nil {
			httpError(w, "Unable to get mesosID: "+err.Error(), http.StatusInternalServerError, r)
			return
		}

		// "/var/lib/mesos/slave/slaves/<mesos_id>/frameworks/<framework_id>/executors/<executor_id>/runs/<container_id>"
		sandboxPath := filepath.Join(sandboxBrowsePath, mesosID, sandboxFrameworks, frameworkID, sandboxExecutors,
			executorID, sandboxRuns, containerID)
		sandboxBaseURL.RawQuery = "path=" + url.QueryEscape(sandboxPath)

		req, err := http.NewRequest("GET", sandboxBaseURL.String(), nil)
		if err != nil {
			httpError(w, "Invalid request: "+err.Error(), http.StatusInternalServerError, r)
			return
		}

		req.Header.Add("Authorization", token)

		resp, err := client.Do(req)
		if err != nil {
			httpError(w, "Could not make auth request: "+err.Error(), http.StatusInternalServerError, r)
			return
		}

		// get a response code and close response body before serving the request.
		responseCode := resp.StatusCode
		resp.Body.Close()

		if responseCode != http.StatusOK {
			httpError(w, fmt.Sprintf("Auth URL %s. Invalid auth response code: %d", sandboxBaseURL.String(), responseCode), responseCode, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Gzip Compression
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func downloadGzippedContentMiddleware(next http.Handler, prefix string, vars ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// log name lazy evaluation
		filenameParts := []string{prefix}
		muxVars := mux.Vars(r)
		for _, v := range vars {
			if muxVar := muxVars[v]; muxVar != "" {
				filenameParts = append(filenameParts, muxVar)
			}
		}

		// get user provided postfix
		if err := r.ParseForm(); err == nil {
			if postfix := r.Form.Get("postfix"); postfix != "" {
				filenameParts = append(filenameParts, postfix)
			}
		}

		filename := strings.Join(filenameParts, "-")
		if filename == "" {
			filename = "download"
		}

		f := fmt.Sprintf("%s.log.gz", filename)
		w.Header().Add("Content-disposition", "attachment; filename="+f)
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzw := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		next.ServeHTTP(gzw, r)
	})
}
