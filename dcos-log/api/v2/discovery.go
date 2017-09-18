package v2

import (
	"net/http"

	"encoding/json"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"strings"
)

const (
	prefix = "/system/v1/agent"
)

type MesosTask struct {
	ID          string
	AgentID     string
	FrameworkID string
	ContainerID string
	ExecutorID  string
}

func (mt MesosTask) validate() error {
	if mt.ContainerID == "" || mt.AgentID == "" || mt.FrameworkID == "" || mt.ExecutorID == "" {
		err := fmt.Errorf("fields cannot be empty: %+v", mt)
		logrus.Error(err)
		return err
	}
	return nil
}

// URL returns a string with URL to an agent that runs the given task.
func (mt MesosTask) URL() (string, error) {
	if err := mt.validate(); err != nil {
		return "", err
	}

	// /system/v1/agent/<agent-id>/logs/v2/stream/<agent-id>/<framework-id>/<executor-id>/<container-id>
	taskLogURL := fmt.Sprintf("%s/%s/logs/v2/task/%s/%s/%s", prefix, mt.AgentID, mt.ContainerID, mt.FrameworkID, mt.ExecutorID)
	logrus.Infof("built URL: %s", taskLogURL)
	return taskLogURL, nil
}

func discover(w http.ResponseWriter, req *http.Request, client *http.Client) {
	taskID := mux.Vars(req)["taskID"]
	if taskID == "" {
		logrus.Error("taskID is empty")
		http.Error(w, "taskID is empty", http.StatusInternalServerError)
		return
	}

	task, err := getTaskDetails(taskID, client)
	if err != nil {
		logrus.Errorf("invalid task id: %s", taskID)
		http.Error(w, "invalid task id: "+taskID, http.StatusBadRequest)
		return
	}

	taskURL, err := task.URL()
	if err != nil {
		logrus.Errorf("unable to validate URL: %s", err)
		http.Error(w, "unable to validate URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, req, taskURL, http.StatusTemporaryRedirect)
}

func getTaskDetails(taskID string, client *http.Client) (*MesosTask, error) {
	resp, err := client.Get("http://leader.mesos:5050/state")
	if err != nil {
		logrus.Errorf("unable to get state from leader.mesos: %s", err)
		return nil, err
	}
	defer resp.Body.Close()

	var mesosResponse MesosResponse
	if err := json.NewDecoder(resp.Body).Decode(&mesosResponse); err != nil {
		return nil, err
	}

	return findTaskInMesosResponse(taskID, mesosResponse)
}

func findTaskInMesosResponse(taskID string, mr MesosResponse) (*MesosTask, error) {
	// find marathon framework
	for _, framewok := range mr.Frameworks {
		if framewok.Name != "marathon" {
			continue
		}

		// find the right task
		for _, task := range framewok.Tasks {
			if task.Name != taskID || !strings.Contains(task.ID, taskID) {
				continue
			}

			mt := &MesosTask{
				AgentID:     task.SlaveID,
				FrameworkID: task.FrameworkID,
				ExecutorID:  task.ID,
			}

			// find container ID
			if len(task.Statuses) == 0 {
				logrus.Errorf("invalid mesos response: %+v", mr)
				return nil, fmt.Errorf("invalid mesos response: %+v", mr)
			}
			mt.ContainerID = task.Statuses[0].ContainerStatus.ContainerID.Value
			return mt, nil
		}
	}

	return nil, fmt.Errorf("task %s not found", taskID)
}

type MesosResponse struct {
	Frameworks []Framework `json:"frameworks"`
}

type Framework struct {
	Name  string `json:"name"`
	Tasks []Task `json:"tasks"`
}

type Task struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	FrameworkID string `json:"framework_id"`
	ExecutorID  string `json:"executor_id"`
	SlaveID     string `json:"slave_id"`
	State       string `json:"state"`

	Statuses []Status `json:"statuses"`
}

type Status struct {
	State           string          `json:"state"`
	ContainerStatus ContainerStatus `json:"container_status"`
}

type ContainerStatus struct {
	ContainerID struct {
		Value string `json:"value"`
	} `json:"container_id"`
}
