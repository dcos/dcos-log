package v2

import (
	"context"
	"net/http"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/dcos/dcos-go/dcos/nodeutil"
)

const (
	prefix = "/system/v1/agent"
)


func redirectURL(id *nodeutil.CanonicalTaskID, file string) (string, error) {
	// find if the task is standalone of a pod.
	isPod := id.ExecutorID != ""
	executorID := id.ExecutorID
	if !isPod {
		executorID = id.ID
	}

	// take the last element
	taskID := id.ContainerIDs[len(id.ContainerIDs) - 1]
	taskLogURL := fmt.Sprintf("%s/%s/logs/v2/task/frameworks/%s/executors/%s/runs/%s/", prefix, id.AgentID,
		id.FrameworkID, executorID, taskID)

	if isPod {
		taskLogURL += fmt.Sprintf("/tasks/%s/%s", id.ID, file)
	} else {
		taskLogURL += file
	}

	return taskLogURL, nil
}

func discover(w http.ResponseWriter, req *http.Request, nodeInfo nodeutil.NodeInfo) {
	vars := mux.Vars(req)
	taskID := vars["taskID"]
	file := vars["file"]

	if file == "" {
		file = "stdout"
	}

	if taskID == "" {
		logrus.Error("taskID is empty")
		http.Error(w, "taskID is empty", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	canonicalTaskID, err := nodeInfo.TaskCanonicalID(ctx, taskID)
	if err != nil {
		errMsg := fmt.Sprintf("unable to get canonical task ID: %s", err)
		logrus.Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	taskURL, err := redirectURL(canonicalTaskID, file)
	if err != nil {
		errMsg := fmt.Sprintf("unable to build redirect URL: %s", err)
		logrus.Error(errMsg)
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, req, taskURL, http.StatusSeeOther)
}

