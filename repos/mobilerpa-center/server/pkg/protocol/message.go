package protocol

const (
	// MessageTypeHello 表示设备首次建立 WebSocket 连接后的握手消息。
	MessageTypeHello = "hello"
	// MessageTypeHeartbeat 表示设备周期上报在线状态的心跳消息。
	MessageTypeHeartbeat = "heartbeat"
	// MessageTypeAssignTask 表示中心向设备下发任务的消息。
	MessageTypeAssignTask = "assign_task"
	// MessageTypeSyncScript 表示中心要求设备同步指定脚本版本的消息。
	MessageTypeSyncScript = "sync_script"
	// MessageTypeTaskAck 表示设备确认收到任务的回执消息。
	MessageTypeTaskAck = "task_ack"
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
