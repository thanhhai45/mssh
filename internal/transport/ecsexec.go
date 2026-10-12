package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ecsExecDialer opens a shell inside a container of an ECS service, through
// `aws ecs execute-command`.
//
// The connection names a service, never a task: a deploy replaces every task,
// so a stored task id would be wrong by the next release. Each Dial looks the
// service up again and lands in its newest healthy task.
//
// ECS Exec runs over Session Manager, so it needs the same two programs as the
// ssm kind, and the same pseudo-terminal for the same reasons.
type ecsExecDialer struct{}

var _ Dialer = ecsExecDialer{}

func (ecsExecDialer) Name() string { return "ECS Exec" }

func (ecsExecDialer) Preflight(config Config) error {
	if _, err := parseECSExecTarget(config); err != nil {
		return err
	}
	if err := requireAWSTools(); err != nil {
		return err
	}
	return checkAWSCredentials(config)
}

func (ecsExecDialer) Dial(
	dialContext context.Context,
	config Config,
	size Size,
	onOutput func([]byte),
	onExit func(error),
) (Session, error) {
	target, err := parseECSExecTarget(config)
	if err != nil {
		return nil, err
	}

	tasks, err := describeServiceTasks(dialContext, config, target)
	if err != nil {
		return nil, err
	}
	chosen, err := pickNewestHealthyTask(tasks, target)
	if err != nil {
		return nil, err
	}

	// Said in the terminal before anything else, because which task this is
	// changes on every deploy, and a shell prompt inside a container rarely
	// tells you.
	onOutput([]byte(chosen.banner(time.Now())))

	command := exec.CommandContext(dialContext, "aws", ecsExecArguments(config, target, chosen)...)
	command.Env = awsEnvironment(pseudoTerminalEnvironment(), config.AWSCredentials, config.AWSRegion)

	return startPTYProcess(command, size, onOutput, onExit, func(output string) string {
		return explainECSExecFailure(chosen.ID, output)
	})
}

// ecsExecTarget is the service an ecs-exec connection points at, and what to
// run once inside.
type ecsExecTarget struct {
	Cluster   string `json:"cluster"`
	Service   string `json:"-"`
	Container string `json:"container"`
	Command   string `json:"command"`
}

const defaultECSExecCommand = "/bin/sh"

// parseECSExecTarget reads the service from Target and the rest from Extra.
func parseECSExecTarget(config Config) (ecsExecTarget, error) {
	var target ecsExecTarget
	if strings.TrimSpace(config.Extra) != "" {
		if err := json.Unmarshal([]byte(config.Extra), &target); err != nil {
			return target, fmt.Errorf("this connection's ECS settings are unreadable: %w", err)
		}
	}
	target.Service = config.Target
	if target.Service == "" {
		return target, fmt.Errorf("this connection has no ECS service")
	}
	if target.Cluster == "" {
		return target, fmt.Errorf("this connection has no ECS cluster")
	}
	if target.Command == "" {
		target.Command = defaultECSExecCommand
	}
	return target, nil
}

// ecsExecArguments is the command line for the interactive session.
func ecsExecArguments(config Config, target ecsExecTarget, task ecsChosenTask) []string {
	return append([]string{
		"ecs", "execute-command",
		"--cluster", target.Cluster,
		"--task", task.ARN,
		"--container", task.Container,
		"--interactive",
		"--command", target.Command,
	}, awsFlags(effectiveProfile(config), config.AWSRegion)...)
}

/* ---------------- finding the task ---------------- */

// ecsTask is the part of describe-tasks this package reads.
type ecsTask struct {
	TaskARN              string         `json:"taskArn"`
	LastStatus           string         `json:"lastStatus"`
	HealthStatus         string         `json:"healthStatus"`
	EnableExecuteCommand bool           `json:"enableExecuteCommand"`
	StartedAt            awsTimestamp   `json:"startedAt"`
	Containers           []ecsContainer `json:"containers"`
}

type ecsContainer struct {
	Name          string `json:"name"`
	LastStatus    string `json:"lastStatus"`
	ManagedAgents []struct {
		Name       string `json:"name"`
		LastStatus string `json:"lastStatus"`
	} `json:"managedAgents"`
}

func (container ecsContainer) execAgentRunning() bool {
	for _, agent := range container.ManagedAgents {
		if agent.Name == "ExecuteCommandAgent" {
			return agent.LastStatus == "RUNNING"
		}
	}
	return false
}

// awsTimestamp reads a timestamp in either form the AWS CLI prints: ISO 8601
// (v2's default) or seconds since the epoch (v1, or cli_timestamp_format).
type awsTimestamp struct{ time.Time }

func (stamp *awsTimestamp) UnmarshalJSON(raw []byte) error {
	text := strings.Trim(string(raw), `"`)
	if text == "" || text == "null" {
		return nil
	}
	if seconds, err := strconv.ParseFloat(text, 64); err == nil {
		stamp.Time = time.Unix(0, int64(seconds*float64(time.Second)))
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return fmt.Errorf("unrecognised AWS timestamp %q", text)
	}
	stamp.Time = parsed
	return nil
}

// describeTasksBatch is the most tasks describe-tasks accepts in one call.
const describeTasksBatch = 100

// describeServiceTasks lists the running tasks of a service and describes them.
func describeServiceTasks(ctx context.Context, config Config, target ecsExecTarget) ([]ecsTask, error) {
	var listed struct {
		TaskARNs []string `json:"taskArns"`
	}
	if err := runAWSJSON(ctx, config, &listed,
		"ecs", "list-tasks",
		"--cluster", target.Cluster,
		"--service-name", target.Service,
		"--desired-status", "RUNNING",
	); err != nil {
		return nil, err
	}
	if len(listed.TaskARNs) == 0 {
		return nil, fmt.Errorf("service %s in cluster %s has no running tasks", target.Service, target.Cluster)
	}

	tasks := []ecsTask{}
	for start := 0; start < len(listed.TaskARNs); start += describeTasksBatch {
		batch := listed.TaskARNs[start:min(start+describeTasksBatch, len(listed.TaskARNs))]
		var described struct {
			Tasks []ecsTask `json:"tasks"`
		}
		arguments := append([]string{"ecs", "describe-tasks", "--cluster", target.Cluster, "--tasks"}, batch...)
		if err := runAWSJSON(ctx, config, &described, arguments...); err != nil {
			return nil, err
		}
		tasks = append(tasks, described.Tasks...)
	}
	return tasks, nil
}

// runAWSJSON runs one aws command and decodes what it prints.
func runAWSJSON(ctx context.Context, config Config, into any, arguments ...string) error {
	callContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	full := append(arguments, "--output", "json")
	full = append(full, awsFlags(effectiveProfile(config), config.AWSRegion)...)
	command := exec.CommandContext(callContext, "aws", full...)
	command.Env = awsEnvironment(pseudoTerminalEnvironment(), config.AWSCredentials, config.AWSRegion)

	var stderr strings.Builder
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		if errors.Is(callContext.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("`aws %s` did not answer within 30 seconds", strings.Join(arguments[:2], " "))
		}
		return errors.New(explainECSLookupFailure(arguments[1], stderr.String()))
	}
	if err := json.Unmarshal(output, into); err != nil {
		return fmt.Errorf("could not read the output of `aws %s`: %w", strings.Join(arguments[:2], " "), err)
	}
	return nil
}

// ecsChosenTask is the task a session goes into, and the container in it.
type ecsChosenTask struct {
	ARN          string
	ID           string
	Container    string
	HealthStatus string
	StartedAt    time.Time
	// Running counts the service's tasks that were running when it was chosen.
	Running int
}

func (task ecsChosenTask) banner(now time.Time) string {
	health := strings.ToLower(task.HealthStatus)
	if task.HealthStatus == "UNKNOWN" || task.HealthStatus == "" {
		health = "no health check"
	}
	age := "start time unknown"
	if !task.StartedAt.IsZero() {
		age = "started " + humanAge(now.Sub(task.StartedAt)) + " ago"
	}
	return fmt.Sprintf("\x1b[2mmssh: task %s · container %s · %s · %s · newest of %d running\x1b[0m\r\n",
		task.ID, task.Container, age, health, task.Running)
}

func humanAge(elapsed time.Duration) string {
	switch {
	case elapsed < time.Minute:
		return fmt.Sprintf("%ds", int(elapsed.Seconds()))
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm", int(elapsed.Minutes()))
	case elapsed < 48*time.Hour:
		return fmt.Sprintf("%dh", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd", int(elapsed.Hours()/24))
	}
}

// pickNewestHealthyTask chooses the task a session goes into.
//
// Only tasks that can take a session at all are candidates: running, with ECS
// Exec turned on, and with the exec agent up in the container. Among those, a
// HEALTHY task wins over one whose health is UNKNOWN, and the newest wins
// within each. UNKNOWN counts, rather than being skipped, because it is all a
// service without a health check ever reports; but during a rolling deploy it
// is also what a brand-new task reports before its first check passes, and
// that is not where somebody debugging production expects to land. UNHEALTHY
// is never chosen.
func pickNewestHealthyTask(tasks []ecsTask, target ecsExecTarget) (ecsChosenTask, error) {
	type candidate struct {
		task      ecsTask
		container string
	}

	running := 0
	healthy, unknown := []candidate{}, []candidate{}
	var execDisabled, agentDown, unhealthy int
	var containerProblem error

	for _, task := range tasks {
		if task.LastStatus != "RUNNING" {
			continue
		}
		running++

		container, err := chooseContainer(task, target.Container)
		if err != nil {
			containerProblem = err
			continue
		}
		switch {
		case !task.EnableExecuteCommand:
			execDisabled++
		case !container.execAgentRunning():
			agentDown++
		case task.HealthStatus == "HEALTHY":
			healthy = append(healthy, candidate{task, container.Name})
		case task.HealthStatus == "UNHEALTHY":
			unhealthy++
		default:
			unknown = append(unknown, candidate{task, container.Name})
		}
	}

	pool := healthy
	if len(pool) == 0 {
		pool = unknown
	}
	if len(pool) == 0 {
		switch {
		case running == 0:
			return ecsChosenTask{}, fmt.Errorf("service %s has no task in the RUNNING state yet", target.Service)
		case containerProblem != nil && execDisabled+agentDown+unhealthy == 0:
			return ecsChosenTask{}, containerProblem
		case execDisabled == running:
			return ecsChosenTask{}, fmt.Errorf(
				"ECS Exec is not enabled on service %s — run `aws ecs update-service --cluster %s "+
					"--service %s --enable-execute-command --force-new-deployment`",
				target.Service, target.Cluster, target.Service)
		case agentDown > 0 && unhealthy == 0:
			return ecsChosenTask{}, fmt.Errorf(
				"the ECS Exec agent is not running in any task of %s yet — a task that just "+
					"started needs a few seconds; otherwise check that the task role allows "+
					"ssmmessages:CreateControlChannel, CreateDataChannel, OpenControlChannel "+
					"and OpenDataChannel", target.Service)
		default:
			return ecsChosenTask{}, fmt.Errorf(
				"no healthy task in %s to connect to (%d running: %d unhealthy, "+
					"%d without the exec agent, %d with ECS Exec off)",
				target.Service, running, unhealthy, agentDown, execDisabled)
		}
	}

	sort.SliceStable(pool, func(first int, second int) bool {
		return pool[first].task.StartedAt.After(pool[second].task.StartedAt.Time)
	})
	chosen := pool[0]
	return ecsChosenTask{
		ARN:          chosen.task.TaskARN,
		ID:           taskIDFromARN(chosen.task.TaskARN),
		Container:    chosen.container,
		HealthStatus: chosen.task.HealthStatus,
		StartedAt:    chosen.task.StartedAt.Time,
		Running:      running,
	}, nil
}

// chooseContainer finds the named container, or the only one when no name was
// given. Guessing between several would put people in the log router or the
// Datadog sidecar as often as in the application.
func chooseContainer(task ecsTask, name string) (ecsContainer, error) {
	names := make([]string, 0, len(task.Containers))
	for _, container := range task.Containers {
		if name != "" && container.Name == name {
			return container, nil
		}
		names = append(names, container.Name)
	}
	if name == "" && len(task.Containers) == 1 {
		return task.Containers[0], nil
	}
	if name == "" {
		return ecsContainer{}, fmt.Errorf(
			"the task runs %d containers (%s) — set which one to open on the connection",
			len(names), strings.Join(names, ", "))
	}
	return ecsContainer{}, fmt.Errorf(
		"the task has no container named %q (it has %s)", name, strings.Join(names, ", "))
}

// taskIDFromARN is the last part of a task ARN, which is what the console shows.
func taskIDFromARN(arn string) string {
	if index := strings.LastIndex(arn, "/"); index >= 0 {
		return arn[index+1:]
	}
	return arn
}

/* ---------------- explaining failures ---------------- */

// explainECSLookupFailure explains a failed list-tasks or describe-tasks.
func explainECSLookupFailure(operation string, output string) string {
	lowered := strings.ToLower(output)
	switch {
	case strings.Contains(lowered, "clusternotfound"):
		return "that ECS cluster does not exist in this account and region"
	case strings.Contains(lowered, "servicenotfound"):
		return "that ECS service does not exist in this cluster"
	case strings.Contains(lowered, "accessdenied"):
		return fmt.Sprintf("this AWS identity is not allowed to call ecs:%s", awsOperationName(operation))
	case strings.Contains(lowered, "expiredtoken"),
		strings.Contains(lowered, "token has expired"):
		return "your AWS session has expired — log in again, or refresh the keys on the workspace"
	default:
		return fmt.Sprintf("aws ecs %s failed: %s", operation, lastLines(output, 3))
	}
}

// awsOperationName turns list-tasks into ListTasks, the name IAM uses.
func awsOperationName(command string) string {
	var name strings.Builder
	for _, part := range strings.Split(command, "-") {
		if part != "" {
			name.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return name.String()
}

// explainECSExecFailure explains an execute-command that started and then
// failed, or a session that ended badly.
func explainECSExecFailure(taskID string, output string) string {
	lowered := strings.ToLower(output)
	switch {
	case strings.Contains(lowered, "targetnotconnected"):
		return fmt.Sprintf(
			"task %s is not reachable through ECS Exec — the exec agent may not be up yet, "+
				"or the task role is missing the ssmmessages permissions", taskID)
	case strings.Contains(lowered, "execute command was not enabled"),
		strings.Contains(lowered, "execute command failed because execute command was not enabled"):
		return fmt.Sprintf("ECS Exec is not enabled on task %s — enable it on the service and redeploy", taskID)
	case strings.Contains(lowered, "accessdenied"):
		return fmt.Sprintf("this AWS identity is not allowed to run ecs:ExecuteCommand on task %s", taskID)
	case strings.Contains(lowered, "oci runtime exec failed"),
		strings.Contains(lowered, "executable file not found"),
		strings.Contains(lowered, "no such file or directory"):
		return "the container has no such command — distroless images often have no " +
			"shell at all; set a command the image does have on the connection"
	}
	if reason, known := ssmFailureReason(taskID, output); known {
		return reason
	}
	return lastLines(output, 3)
}
