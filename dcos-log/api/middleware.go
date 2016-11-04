package api

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
	// authCookieName is a token name passed by a browser in `Cookie` request header.
	authCookieName = "dcos-acs-auth-cookie"

	sandboxURLScheme  = "https"
	sandboxPath       = "/files/browse"
	sandboxBrowsePath = "/var/lib/mesos/slave/slaves"
	sandboxFrameworks = "frameworks"
	sandboxExecutors  = "executors"
	sandboxRuns       = "runs"
)

var (
	// ErrMissingToken is returned by getAuthOrCookieFromRequest when JWT is missing.
	ErrMissingToken = errors.New("Missing token in auth request")

	// ErrInvalidToken is returned by validateToken if a token string did not pass validation.
	ErrInvalidToken = errors.New("Invalid token used")
)

func getSandboxURL(nodeInfo nodeutil.NodeInfo) (*url.URL, error) {
	role, err := nodeInfo.Role()
	if err != nil {
		return nil, err
	}

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
		return t, ErrInvalidToken
	}

	// JWT structure: header.payload.signature https://jwt.io/introduction/
	if parts := strings.Split(t, "."); len(parts) != 3 {
		return t, ErrInvalidToken
	}

	// TODO: improve token validation
	return t, nil
}

// getAuthOrCookieFromRequest will try to extract JWT from Authorization header first. If it is not there
// it will try to look for a dcos-acs-auth-cookie.
func getAuthOrCookieFromRequest(r *http.Request) (string, error) {
	// give priority to Authorization header
	authorizationHeader := r.Header.Get("Authorization")
	if authorizationHeader != "" {
		return validateToken(authorizationHeader)
	}

	// if authorization header is not set and no cookie header, return empty string
	cookieHeader := r.Header.Get("Cookie")
	if cookieHeader == "" {
		return "", ErrMissingToken
	}

	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		keyValue := strings.Split(cookie, "=")
		if strings.TrimSpace(keyValue[0]) == authCookieName {
			return validateToken("token=" + keyValue[1])
		}
	}

	return "", ErrMissingToken
}

func authMiddleware(next http.Handler, client *http.Client, nodeInfo nodeutil.NodeInfo) http.Handler {
	if nodeInfo == nil {
		panic("nodeInfo cannot be nil")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// JWT is required to present in a request. The middleware will extract the token and try to access
		// sandbox with it. We authorize the request if sandbox returns 200.
		token, err := getAuthOrCookieFromRequest(r)
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

		sandboxBaseURL, err := getSandboxURL(nodeInfo)
		if err != nil {
			httpError(w, "Unable to get sandboxBaseURL: "+err.Error(), http.StatusInternalServerError, r)
			return
		}

		mesosID, err := nodeInfo.MesosID(nil)
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
