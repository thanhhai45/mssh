package transport

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

/* ---------------- choosing the task ---------------- */

// fakeTask is a task that can take a session: running, exec on, agent up.
func fakeTask(id string, health string, startedAt time.Time) ecsTask {
	task := ecsTask{
		TaskARN:              "arn:aws:ecs:ap-southeast-1:111122223333:task/prod/" + id,
		LastStatus:           "RUNNING",
		HealthStatus:         health,
		EnableExecuteCommand: true,
		StartedAt:            awsTimestamp{startedAt},
		Containers:           []ecsContainer{fakeContainer("app", "RUNNING")},
	}
	return task
}

func fakeContainer(name string, agentStatus string) ecsContainer {
	container := ecsContainer{Name: name, LastStatus: "RUNNING"}
	container.ManagedAgents = append(container.ManagedAgents, struct {
		Name       string `json:"name"`
		LastStatus string `json:"lastStatus"`
	}{"ExecuteCommandAgent", agentStatus})
	return container
}

func TestPickNewestHealthyTask(t *testing.T) {
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	target := ecsExecTarget{Cluster: "prod", Service: "api"}

	tests := []struct {
		name    string
		tasks   []ecsTask
		target  ecsExecTarget
		wantID  string
		wantErr string
	}{
		{
			name: "the newest healthy task wins",
			tasks: []ecsTask{
				fakeTask("old", "HEALTHY", base),
				fakeTask("new", "HEALTHY", base.Add(time.Hour)),
				fakeTask("middle", "HEALTHY", base.Add(time.Minute)),
			},
			wantID: "new",
		},
		{
			name: "a newer task still starting loses to an older healthy one",
			tasks: []ecsTask{
				fakeTask("healthy", "HEALTHY", base),
				fakeTask("starting", "UNKNOWN", base.Add(time.Hour)),
			},
			wantID: "healthy",
		},
		{
			name: "without a health check, the newest task wins",
			tasks: []ecsTask{
				fakeTask("old", "UNKNOWN", base),
				fakeTask("new", "UNKNOWN", base.Add(time.Hour)),
			},
			wantID: "new",
		},
		{
			name: "an unhealthy task is never chosen",
			tasks: []ecsTask{
				fakeTask("unknown", "UNKNOWN", base),
				fakeTask("unhealthy", "UNHEALTHY", base.Add(time.Hour)),
			},
			wantID: "unknown",
		},
		{
			name: "a task whose exec agent is down is skipped",
			tasks: func() []ecsTask {
				down := fakeTask("down", "HEALTHY", base.Add(time.Hour))
				down.Containers = []ecsContainer{fakeContainer("app", "PENDING")}
				return []ecsTask{fakeTask("up", "HEALTHY", base), down}
			}(),
			wantID: "up",
		},
		{
			name: "a stopping task is skipped",
			tasks: func() []ecsTask {
				stopping := fakeTask("stopping", "HEALTHY", base.Add(time.Hour))
				stopping.LastStatus = "DEACTIVATING"
				return []ecsTask{fakeTask("running", "HEALTHY", base), stopping}
			}(),
			wantID: "running",
		},
		{
			name: "several containers need a name",
			tasks: func() []ecsTask {
				task := fakeTask("t", "HEALTHY", base)
				task.Containers = append(task.Containers, fakeContainer("datadog-agent", "RUNNING"))
				return []ecsTask{task}
			}(),
			wantErr: "set which one to open",
		},
		{
			name: "the named container is used",
			tasks: func() []ecsTask {
				task := fakeTask("t", "HEALTHY", base)
				task.Containers = append([]ecsContainer{fakeContainer("datadog-agent", "RUNNING")},
					task.Containers...)
				return []ecsTask{task}
			}(),
			target: ecsExecTarget{Cluster: "prod", Service: "api", Container: "app"},
			wantID: "t",
		},
		{
			name: "exec turned off says how to turn it on",
			tasks: func() []ecsTask {
				task := fakeTask("t", "HEALTHY", base)
				task.EnableExecuteCommand = false
				return []ecsTask{task}
			}(),
			wantErr: "--enable-execute-command",
		},
		{
			name:    "only unhealthy tasks is an error that counts them",
			tasks:   []ecsTask{fakeTask("bad", "UNHEALTHY", base)},
			wantErr: "1 unhealthy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.target.Service == "" {
				tc.target = target
			}
			got, err := pickNewestHealthyTask(tc.tasks, tc.target)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("pick failed: %v", err)
			}
			if got.ID != tc.wantID {
				t.Errorf("chose %s, want %s", got.ID, tc.wantID)
			}
			if got.Container != "app" {
				t.Errorf("container = %q, want app", got.Container)
			}
		})
	}
}

func TestAWSTimestampReadsBothForms(t *testing.T) {
	want := time.Date(2026, 10, 1, 9, 30, 15, 0, time.UTC)
	for _, raw := range []string{`"2026-10-01T16:30:15+07:00"`, `"2026-10-01T09:30:15.000000+00:00"`, `1790847015`} {
		var stamp awsTimestamp
		if err := json.Unmarshal([]byte(raw), &stamp); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if !stamp.Equal(want) {
			t.Errorf("%s = %v, want %v", raw, stamp.Time, want)
		}
	}
}

func TestECSExecArguments(t *testing.T) {
	config := Config{AWSProfile: "dev", AWSRegion: "ap-southeast-1"}
	target := ecsExecTarget{Cluster: "prod", Service: "api", Command: "/bin/bash"}
	task := ecsChosenTask{ARN: "arn:aws:ecs:x:1:task/prod/abc", Container: "app"}

	got := strings.Join(ecsExecArguments(config, target, task), " ")
	want := "ecs execute-command --cluster prod --task arn:aws:ecs:x:1:task/prod/abc " +
		"--container app --interactive --command /bin/bash --profile dev --region ap-southeast-1"
	if got != want {
		t.Errorf("arguments:\n got %s\nwant %s", got, want)
	}
}

func TestParseECSExecTarget(t *testing.T) {
	got, err := parseECSExecTarget(Config{Target: "api", Extra: `{"cluster":"prod"}`})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Service != "api" || got.Cluster != "prod" || got.Command != defaultECSExecCommand {
		t.Errorf("got %+v", got)
	}
	if _, err := parseECSExecTarget(Config{Target: "api", Extra: `{}`}); err == nil {
		t.Error("a connection without a cluster was accepted")
	}
}

/* ---------------- against the fake aws ---------------- */

// fakeECSLookup answers list-tasks and describe-tasks from MSSH_FAKE_ECS_TASKS,
// a describe-tasks reply, or fails with MSSH_FAKE_ECS_FAILURE.
func fakeECSLookup(command string) int {
	if failure := os.Getenv("MSSH_FAKE_ECS_FAILURE"); failure != "" {
		_, _ = fmt.Fprintln(os.Stderr, failure)
		return 254
	}
	var reply struct {
		Tasks []map[string]any `json:"tasks"`
	}
	_ = json.Unmarshal([]byte(os.Getenv("MSSH_FAKE_ECS_TASKS")), &reply)
	if command == "ecs list-tasks" {
		arns := []string{}
		for _, task := range reply.Tasks {
			arns = append(arns, task["taskArn"].(string))
		}
		encoded, _ := json.Marshal(map[string]any{"taskArns": arns})
		fmt.Println(string(encoded))
		return 0
	}
	encoded, _ := json.Marshal(reply)
	fmt.Println(string(encoded))
	return 0
}

const fakeDescribeTasks = `{"tasks": [
  {"taskArn": "arn:aws:ecs:ap-southeast-1:111122223333:task/prod/older", "lastStatus": "RUNNING",
   "healthStatus": "HEALTHY", "enableExecuteCommand": true, "startedAt": "2026-10-01T09:00:00+00:00",
   "containers": [{"name": "app", "lastStatus": "RUNNING",
     "managedAgents": [{"name": "ExecuteCommandAgent", "lastStatus": "RUNNING"}]}]},
  {"taskArn": "arn:aws:ecs:ap-southeast-1:111122223333:task/prod/newer", "lastStatus": "RUNNING",
   "healthStatus": "HEALTHY", "enableExecuteCommand": true, "startedAt": "2026-10-01T10:00:00+00:00",
   "containers": [{"name": "app", "lastStatus": "RUNNING",
     "managedAgents": [{"name": "ExecuteCommandAgent", "lastStatus": "RUNNING"}]}]}
]}`

func TestECSExecLandsInTheNewestHealthyTask(t *testing.T) {
	argsLog, _ := useFakeAWS(t, fakeAWS{}, true)
	t.Setenv("MSSH_FAKE_ECS_TASKS", fakeDescribeTasks)
	t.Setenv("MSSH_FAKE_ECS_FAILURE", "")

	config := Config{Kind: KindECSExec, Target: "api", Extra: `{"cluster":"prod"}`, AWSRegion: "ap-southeast-1"}
	dialer, err := For(KindECSExec)
	if err != nil {
		t.Fatal(err)
	}
	if err := dialer.Preflight(config); err != nil {
		t.Fatalf("preflight: %v", err)
	}

	log := newSessionLog()
	session, err := dialer.Dial(t.Context(), config, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	log.waitFor(t, "mssh: task newer · container app")
	log.waitFor(t, "fake-ssm$ ")
	_ = session.Close()
	if err := log.waitExit(t); err != nil {
		t.Errorf("closing reported %v", err)
	}

	recorded := calls(t, argsLog)
	exec := slices.IndexFunc(recorded, func(line string) bool {
		return strings.HasPrefix(line, "ecs execute-command")
	})
	if exec < 0 {
		t.Fatalf("execute-command never ran; calls: %q", recorded)
	}
	if !strings.Contains(recorded[exec], "--task arn:aws:ecs:ap-southeast-1:111122223333:task/prod/newer") {
		t.Errorf("went into the wrong task: %s", recorded[exec])
	}
}

func TestECSExecExplainsAMissingService(t *testing.T) {
	useFakeAWS(t, fakeAWS{}, true)
	t.Setenv("MSSH_FAKE_ECS_FAILURE",
		"An error occurred (ServiceNotFoundException) when calling the ListTasks operation: Service not found.")

	config := Config{Kind: KindECSExec, Target: "api", Extra: `{"cluster":"prod"}`}
	log := newSessionLog()
	_, err := ecsExecDialer{}.Dial(t.Context(), config, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit)
	if err == nil || !strings.Contains(err.Error(), "service does not exist") {
		t.Fatalf("err = %v, want it to say the service does not exist", err)
	}
}
