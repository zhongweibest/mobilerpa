package workflow

import (
	"context"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
)

type testDispatcher struct {
	tasks *task.Service
}

func (d testDispatcher) AssignTask(ctx context.Context, taskID string) (task.Task, error) {
	if d.tasks == nil {
		return task.Task{}, fmt.Errorf("test dispatcher tasks is nil")
	}
	return d.tasks.MarkAssigned(ctx, taskID, "test-dispatch")
}

func newTestWorkflowService(t *testing.T) (*Service, *device.Service, *task.Service) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "workflow-service-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	return NewService(db, deviceService, taskService, testDispatcher{tasks: taskService}), deviceService, taskService
}

func registerWorkflowTestDevice(t *testing.T, ctx context.Context, devices *device.Service, agentUUID string) string {
	t.Helper()

	req := httptest.NewRequest("POST", "http://127.0.0.1/api/v1/device/register", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	item, err := devices.Register(ctx, device.RegisterRequest{
		AgentUUID:  agentUUID,
		DeviceName: agentUUID,
		Brand:      "Google",
		Model:      "Pixel 8",
	}, req)
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if err := devices.UpdateExecutionProfile(ctx, item.DeviceID, device.ExecutionProfile{
		AccessibilityStatus:              "enabled",
		ForegroundServiceStatus:          "enabled",
		BatteryOptimizationIgnoredStatus: "enabled",
		CheckedAt:                        time.Now().UTC().Format(time.RFC3339),
		Message:                          "test ready",
	}); err != nil {
		t.Fatalf("update execution profile: %v", err)
	}
	return item.DeviceID
}

func createWorkflowDefinitionForTest(t *testing.T, ctx context.Context, service *Service) Definition {
	t.Helper()

	item, err := service.CreateDefinition(ctx, CreateDefinitionRequest{
		WorkflowName: "测试工作流",
		Status:       DefinitionStatusActive,
		Nodes: []Node{
			{
				NodeID:        "node_a",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤A",
				ScriptName:    "open_qq",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:   "node_stop",
				NodeType: NodeTypeStop,
				NodeName: "结束",
			},
		},
		Edges: []Edge{
			{
				FromNodeID: "node_a",
				ToNodeID:   "node_stop",
				EdgeType:   EdgeTypeNext,
			},
		},
	})
	if err != nil {
		t.Fatalf("create workflow definition: %v", err)
	}
	return item
}

func createLoopWorkflowDefinitionForTest(t *testing.T, ctx context.Context, service *Service, maxIterations int) Definition {
	t.Helper()

	item, err := service.CreateDefinition(ctx, CreateDefinitionRequest{
		WorkflowName: "循环工作流测试",
		Status:       DefinitionStatusActive,
		Nodes: []Node{
			{
				NodeID:        "node_a",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤A",
				ScriptName:    "open_qq",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:        "node_loop",
				NodeType:      NodeTypeLoop,
				NodeName:      "循环节点",
				MaxIterations: maxIterations,
			},
			{
				NodeID:        "node_b",
				NodeType:      NodeTypeScript,
				NodeName:      "循环体",
				ScriptName:    "open_wechat",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:   "node_stop",
				NodeType: NodeTypeStop,
				NodeName: "结束",
			},
		},
		Edges: []Edge{
			{
				FromNodeID: "node_a",
				ToNodeID:   "node_loop",
				EdgeType:   EdgeTypeNext,
			},
			{
				FromNodeID: "node_loop",
				ToNodeID:   "node_b",
				EdgeType:   EdgeTypeLoopBody,
			},
			{
				FromNodeID: "node_b",
				ToNodeID:   "node_loop",
				EdgeType:   EdgeTypeNext,
			},
			{
				FromNodeID: "node_loop",
				ToNodeID:   "node_stop",
				EdgeType:   EdgeTypeLoopExit,
			},
		},
	})
	if err != nil {
		t.Fatalf("create loop workflow definition: %v", err)
	}
	return item
}

func createMultiSegmentWorkflowDefinitionForTest(t *testing.T, ctx context.Context, service *Service) Definition {
	t.Helper()

	item, err := service.CreateDefinition(ctx, CreateDefinitionRequest{
		WorkflowName: "多段循环工作流测试",
		Status:       DefinitionStatusActive,
		Nodes: []Node{
			{
				NodeID:        "node_a",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤A",
				ScriptName:    "open_qq",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:        "node_b",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤B",
				ScriptName:    "open_wechat",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:        "node_loop_1",
				NodeType:      NodeTypeLoop,
				NodeName:      "循环段1",
				MaxIterations: 3,
			},
			{
				NodeID:        "node_c",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤C",
				ScriptName:    "open_qq",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:        "node_d",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤D",
				ScriptName:    "open_wechat",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:        "node_e",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤E",
				ScriptName:    "open_qq",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:        "node_loop_2",
				NodeType:      NodeTypeLoop,
				NodeName:      "循环段2",
				MaxIterations: 5,
			},
			{
				NodeID:        "node_f",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤F",
				ScriptName:    "open_wechat",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:        "node_g",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤G",
				ScriptName:    "open_qq",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:        "node_h",
				NodeType:      NodeTypeScript,
				NodeName:      "步骤H",
				ScriptName:    "open_wechat",
				ScriptVersion: "v0.1.0",
			},
			{
				NodeID:   "node_stop",
				NodeType: NodeTypeStop,
				NodeName: "结束",
			},
		},
		Edges: []Edge{
			{FromNodeID: "node_a", ToNodeID: "node_b", EdgeType: EdgeTypeNext},
			{FromNodeID: "node_b", ToNodeID: "node_loop_1", EdgeType: EdgeTypeNext},
			{FromNodeID: "node_loop_1", ToNodeID: "node_c", EdgeType: EdgeTypeLoopBody},
			{FromNodeID: "node_c", ToNodeID: "node_d", EdgeType: EdgeTypeNext},
			{FromNodeID: "node_d", ToNodeID: "node_e", EdgeType: EdgeTypeNext},
			{FromNodeID: "node_e", ToNodeID: "node_loop_1", EdgeType: EdgeTypeNext},
			{FromNodeID: "node_loop_1", ToNodeID: "node_loop_2", EdgeType: EdgeTypeLoopExit},
			{FromNodeID: "node_loop_2", ToNodeID: "node_f", EdgeType: EdgeTypeLoopBody},
			{FromNodeID: "node_f", ToNodeID: "node_g", EdgeType: EdgeTypeNext},
			{FromNodeID: "node_g", ToNodeID: "node_loop_2", EdgeType: EdgeTypeNext},
			{FromNodeID: "node_loop_2", ToNodeID: "node_h", EdgeType: EdgeTypeLoopExit},
			{FromNodeID: "node_h", ToNodeID: "node_stop", EdgeType: EdgeTypeNext},
		},
	})
	if err != nil {
		t.Fatalf("create multi segment workflow definition: %v", err)
	}
	return item
}

func markWorkflowRunCurrentTaskSuccess(t *testing.T, ctx context.Context, taskService *task.Service, workflowService *Service, workflowRunID string, requestIDSuffix string) task.Task {
	t.Helper()

	run, err := workflowService.getRunByID(ctx, workflowRunID)
	if err != nil {
		t.Fatalf("get workflow run: %v", err)
	}
	if strings.TrimSpace(run.CurrentTaskID) == "" {
		t.Fatalf("workflow run %s current_task_id is empty", workflowRunID)
	}

	taskItem, err := taskService.MarkResult(ctx, run.CurrentTaskID, task.ResultPayload{
		Status:      task.StatusSuccess,
		ResultCode:  "ok",
		ResultMessage: "测试成功",
	}, "test-result-"+requestIDSuffix)
	if err != nil {
		t.Fatalf("mark workflow task result success: %v", err)
	}
	if err := workflowService.HandleTaskResult(ctx, taskItem.TaskID); err != nil {
		t.Fatalf("handle workflow task result: %v", err)
	}
	return taskItem
}

func TestLoopWorkflowRunsExactMaxIterations(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	service, devices, tasks := newTestWorkflowService(t)
	deviceID := registerWorkflowTestDevice(t, ctx, devices, "agent-loop-001")
	definition := createLoopWorkflowDefinitionForTest(t, ctx, service, 2)

	instance, err := service.Start(ctx, definition.WorkflowDefID, StartRequest{
		DeviceIDs: []string{deviceID},
	})
	if err != nil {
		t.Fatalf("start loop workflow: %v", err)
	}
	if len(instance.DeviceRuns) != 1 {
		t.Fatalf("unexpected device run count: %d", len(instance.DeviceRuns))
	}

	workflowRunID := instance.DeviceRuns[0].WorkflowRunID

	markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, "node-a")

	run, err := service.getRunByID(ctx, workflowRunID)
	if err != nil {
		t.Fatalf("get workflow run after node_a: %v", err)
	}
	if run.CurrentNodeID != "node_b" {
		t.Fatalf("expected current node to enter loop body node_b, got %s", run.CurrentNodeID)
	}

	markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, "node-b-1")
	markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, "node-b-2")

	run, err = service.getRunByID(ctx, workflowRunID)
	if err != nil {
		t.Fatalf("get workflow run after loop completion: %v", err)
	}
	if run.Status != RunStatusSuccess {
		t.Fatalf("expected workflow run success, got %s", run.Status)
	}
	if run.CurrentNodeID != "node_stop" {
		t.Fatalf("expected workflow run stop on node_stop, got %s", run.CurrentNodeID)
	}

	events, err := service.ListEvents(ctx, definition.WorkflowDefID, workflowRunID)
	if err != nil {
		t.Fatalf("list workflow events: %v", err)
	}

	loopCompletedCount := 0
	nodeBStartedCount := 0
	loopCounters := make([]int, 0)
	for _, item := range events {
		if item.EventType == EventTypeWorkflowLoopCompleted {
			loopCompletedCount += 1
			counterValue, ok := item.Extra["counter"].(float64)
			if !ok {
				t.Fatalf("expected loop counter as float64 in event extra, got %#v", item.Extra["counter"])
			}
			loopCounters = append(loopCounters, int(counterValue))
		}
		if item.EventType == EventTypeWorkflowStepStarted && item.NodeID == "node_b" {
			nodeBStartedCount += 1
		}
	}

	if loopCompletedCount != 2 {
		t.Fatalf("expected 2 loop completion events for max_iterations=2, got %d", loopCompletedCount)
	}
	if nodeBStartedCount != 2 {
		t.Fatalf("expected loop body node_b to execute exactly 2 times, got %d", nodeBStartedCount)
	}
	if len(loopCounters) != 2 || loopCounters[0] != 2 || loopCounters[1] != 1 {
		t.Fatalf("unexpected loop counters: %#v", loopCounters)
	}

	counter, err := service.getLoopCounter(ctx, workflowRunID, "node_loop")
	if err != nil {
		t.Fatalf("get loop counter: %v", err)
	}
	if counter != 2 {
		t.Fatalf("expected stored loop counter to be 2 after loop completion, got %d", counter)
	}
}

func TestMultiSegmentWorkflowRunsThroughAllSegments(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	service, devices, tasks := newTestWorkflowService(t)
	deviceID := registerWorkflowTestDevice(t, ctx, devices, "agent-multi-segment-001")
	definition := createMultiSegmentWorkflowDefinitionForTest(t, ctx, service)

	instance, err := service.Start(ctx, definition.WorkflowDefID, StartRequest{
		DeviceIDs: []string{deviceID},
	})
	if err != nil {
		t.Fatalf("start multi segment workflow: %v", err)
	}
	if len(instance.DeviceRuns) != 1 {
		t.Fatalf("unexpected device run count: %d", len(instance.DeviceRuns))
	}

	workflowRunID := instance.DeviceRuns[0].WorkflowRunID

	markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, "node-a")
	markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, "node-b")

	for index := 0; index < 3; index += 1 {
		markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, fmt.Sprintf("node-c-%d", index))
		markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, fmt.Sprintf("node-d-%d", index))
		markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, fmt.Sprintf("node-e-%d", index))
	}

	for index := 0; index < 5; index += 1 {
		markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, fmt.Sprintf("node-f-%d", index))
		markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, fmt.Sprintf("node-g-%d", index))
	}

	markWorkflowRunCurrentTaskSuccess(t, ctx, tasks, service, workflowRunID, "node-h")

	run, err := service.getRunByID(ctx, workflowRunID)
	if err != nil {
		t.Fatalf("get workflow run after all segments: %v", err)
	}
	if run.Status != RunStatusSuccess {
		t.Fatalf("expected workflow run success, got %s", run.Status)
	}
	if run.CurrentNodeID != "node_stop" {
		t.Fatalf("expected workflow run stop on node_stop, got %s", run.CurrentNodeID)
	}

	loopCounter1, err := service.getLoopCounter(ctx, workflowRunID, "node_loop_1")
	if err != nil {
		t.Fatalf("get loop counter 1: %v", err)
	}
	if loopCounter1 != 3 {
		t.Fatalf("expected loop 1 counter to be 3, got %d", loopCounter1)
	}

	loopCounter2, err := service.getLoopCounter(ctx, workflowRunID, "node_loop_2")
	if err != nil {
		t.Fatalf("get loop counter 2: %v", err)
	}
	if loopCounter2 != 5 {
		t.Fatalf("expected loop 2 counter to be 5, got %d", loopCounter2)
	}

	events, err := service.ListEvents(ctx, definition.WorkflowDefID, workflowRunID)
	if err != nil {
		t.Fatalf("list workflow events: %v", err)
	}

	loop1Count := 0
	loop2Count := 0
	nodeHStarted := false
	completed := false
	for _, item := range events {
		if item.EventType == EventTypeWorkflowLoopCompleted && item.NodeID == "node_loop_1" {
			loop1Count += 1
		}
		if item.EventType == EventTypeWorkflowLoopCompleted && item.NodeID == "node_loop_2" {
			loop2Count += 1
		}
		if item.EventType == EventTypeWorkflowStepStarted && item.NodeID == "node_h" {
			nodeHStarted = true
		}
		if item.EventType == EventTypeWorkflowRunCompleted && item.NodeID == "node_stop" {
			completed = true
		}
	}

	if loop1Count != 3 {
		t.Fatalf("expected loop 1 completed count to be 3, got %d", loop1Count)
	}
	if loop2Count != 5 {
		t.Fatalf("expected loop 2 completed count to be 5, got %d", loop2Count)
	}
	if !nodeHStarted {
		t.Fatalf("expected node_h step started event")
	}
	if !completed {
		t.Fatalf("expected workflow completed event on node_stop")
	}
}
