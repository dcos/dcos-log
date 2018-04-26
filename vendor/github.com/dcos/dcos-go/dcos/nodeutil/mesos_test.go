package nodeutil

import (
	"strings"
	"testing"
)

func TestTaskContainerID(t *testing.T) {
	status := Status{
		ContainerStatus: ContainerStatus{
			ContainerID: NestedValue{
				Value: "test",
			},
		},
	}

	task := Task{
		Statuses: []Status{status},
	}

	ids, err := task.ContainerIDs()
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(ids, "") != "test" {
		t.Fatalf("expect container id test. Got %s", ids)
	}
}

func TestTaskNestedContainerID(t *testing.T) {
	status := Status{
		ContainerStatus: ContainerStatus{
			ContainerID: NestedValue{
				Value: "test",
				Parent: &NestedValue{
					Value: "test2",
				},
			},
		},
	}

	task := Task{
		Statuses: []Status{status},
	}

	ids, err := task.ContainerIDs()
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(ids, "-") != "test-test2" {
		t.Fatalf("expect 2 ids: test, test2. Got %s", ids)
	}
}

func TestTaskNestedContainerID2(t *testing.T) {
	status := Status{
		ContainerStatus: ContainerStatus{
			ContainerID: NestedValue{
				Value: "test",
				Parent: &NestedValue{
					Value: "test2",
					Parent: &NestedValue{
						Value: "test3",
					},
				},
			},
		},
	}

	task := Task{
		Statuses: []Status{status},
	}

	ids, err := task.ContainerIDs()
	if err != nil {
		t.Fatal(err)
	}

	if strings.Join(ids, "-") != "test-test2-test3" {
		t.Fatalf("expect 2 ids: test, test2, test3. Got %s", ids)
	}
}
