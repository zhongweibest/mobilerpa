package protocol

const (
	// MessageTypeHello 表示设备首次建立 WebSocket 连接后的握手消息。
	MessageTypeHello = "hello"
	// MessageTypeHeartbeat 表示设备周期上报在线状态的心跳消息。
	MessageTypeHeartbeat = "heartbeat"
	// MessageTypeAssignTask 表示中心向设备下发任务的消息。
	MessageTypeAssignTask = "assign_task"
	// MessageTypeStartWorkflowSession 表示中心向设备下发工作流会话。
	MessageTypeStartWorkflowSession = "start_workflow_session"
	// MessageTypeStopWorkflowSession 表示中心要求设备停止工作流会话。
	MessageTypeStopWorkflowSession = "stop_workflow_session"
	// MessageTypeSyncScript 表示中心要求设备同步指定脚本版本的消息。
	MessageTypeSyncScript = "sync_script"
	// MessageTypeTaskAck 表示设备确认收到任务的回执消息。
	MessageTypeTaskAck = "task_ack"
	// MessageTypeWorkflowSessionAck 表示设备确认收到工作流会话的回执消息。
	MessageTypeWorkflowSessionAck = "workflow_session_ack"
	// MessageTypeWorkflowSessionEvent 表示设备回传工作流会话关键事件。
	MessageTypeWorkflowSessionEvent = "workflow_session_event"
	// MessageTypeWorkflowSessionResult 表示设备回传工作流会话最终结果。
	MessageTypeWorkflowSessionResult = "workflow_session_result"
	// MessageTypeScriptSyncAck 表示设备确认收到脚本同步指令的回执消息。
	MessageTypeScriptSyncAck = "script_sync_ack"
	// MessageTypeScriptSyncResult 表示设备回传脚本同步结果的消息。
	MessageTypeScriptSyncResult = "script_sync_result"
	// MessageTypeTaskProgress 表示设备回传任务执行过程中的关键步骤事件。
	MessageTypeTaskProgress = "task_progress"
	// MessageTypeWorkflowStepProgress 表示设备回传工作流步骤过程中的关键进度事件。
	MessageTypeWorkflowStepProgress = "workflow_step_progress"
	// MessageTypeTaskResult 表示设备回传任务执行结果的消息。
	MessageTypeTaskResult = "task_result"
)

// Envelope 是中心与设备之间共用的 WebSocket JSON 外层结构。
type Envelope struct {
	// Type 是消息类型，例如 hello、heartbeat、assign_task、task_ack、task_result 或 ack。
	Type string `json:"type"`
	// RequestID 用于在 WebSocket 通道中关联请求和响应。
	RequestID string `json:"request_id"`
	// DeviceID 是消息关联的中心设备标识。
	DeviceID string `json:"device_id"`
	// Timestamp 是发送方上报的 Unix 时间戳。
	Timestamp int64 `json:"timestamp"`
	// Payload 是具体消息类型对应的载荷。
	Payload any `json:"payload"`
}

// StartWorkflowSessionPayload 描述中心下发给设备的工作流会话。
type StartWorkflowSessionPayload struct {
	WorkflowSessionID  string                   `json:"workflow_session_id"`
	WorkflowInstanceID string                   `json:"workflow_instance_id"`
	WorkflowRunID      string                   `json:"workflow_run_id"`
	WorkflowDefID      string                   `json:"workflow_def_id"`
	WorkflowName       string                   `json:"workflow_name"`
	DeviceID           string                   `json:"device_id"`
	EntryNodeID        string                   `json:"entry_node_id"`
	DefinitionSnapshot WorkflowDefinitionSnapshot `json:"definition_snapshot"`
	ScriptManifest     []WorkflowScriptManifest `json:"script_manifest"`
	RuntimePolicy      map[string]any           `json:"runtime_policy"`
}

// StopWorkflowSessionPayload 描述中心要求设备停止某次工作流会话。
type StopWorkflowSessionPayload struct {
	WorkflowSessionID string         `json:"workflow_session_id"`
	WorkflowInstanceID string        `json:"workflow_instance_id"`
	WorkflowRunID     string         `json:"workflow_run_id"`
	WorkflowDefID     string         `json:"workflow_def_id"`
	DeviceID          string         `json:"device_id"`
	Reason            string         `json:"reason"`
	Extra             map[string]any `json:"extra"`
}

// WorkflowDefinitionSnapshot 描述某次运行冻结后的工作流定义。
type WorkflowDefinitionSnapshot struct {
	Nodes []WorkflowNodeSnapshot `json:"nodes"`
	Edges []WorkflowEdgeSnapshot `json:"edges"`
}

// WorkflowNodeSnapshot 描述冻结后的工作流节点。
type WorkflowNodeSnapshot struct {
	NodeID        string `json:"node_id"`
	NodeType      string `json:"node_type"`
	NodeName      string `json:"node_name"`
	ScriptName    string `json:"script_name"`
	ScriptVersion string `json:"script_version"`
	MaxIterations int    `json:"max_iterations"`
	Position      int    `json:"position"`
}

// WorkflowEdgeSnapshot 描述冻结后的工作流边。
type WorkflowEdgeSnapshot struct {
	FromNodeID string `json:"from_node_id"`
	ToNodeID   string `json:"to_node_id"`
	EdgeType   string `json:"edge_type"`
}

// WorkflowScriptManifest 描述本次工作流会话依赖的脚本版本。
type WorkflowScriptManifest struct {
	ScriptName    string `json:"script_name"`
	ScriptVersion string `json:"script_version"`
}

// WorkflowSessionAckPayload 描述设备对工作流会话的确认回执。
type WorkflowSessionAckPayload struct {
	WorkflowRunID string `json:"workflow_run_id"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

// WorkflowSessionEventPayload 描述设备回传的工作流会话关键事件。
type WorkflowSessionEventPayload struct {
	WorkflowRunID string         `json:"workflow_run_id"`
	WorkflowNodeID string        `json:"workflow_node_id"`
	EventType     string         `json:"event_type"`
	Status        string         `json:"status"`
	StepName      string         `json:"step_name"`
	Message       string         `json:"message"`
	Extra         map[string]any `json:"extra"`
}

// WorkflowSessionResultPayload 描述设备回传的工作流会话最终结果。
type WorkflowSessionResultPayload struct {
	WorkflowRunID string         `json:"workflow_run_id"`
	WorkflowNodeID string        `json:"workflow_node_id"`
	Status        string         `json:"status"`
	ResultCode    string         `json:"result_code"`
	ResultMessage string         `json:"result_message"`
	Extra         map[string]any `json:"extra"`
}
