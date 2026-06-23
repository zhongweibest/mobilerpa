package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
	"github.com/mobilerpa/mobilerpa-center/server/pkg/protocol"
)

// Handler 管理设备 Agent 使用的 WebSocket 接入端点。
type Handler struct {
	devices    *device.Service
	dispatcher *dispatch.Service
	workflows  *workflow.Service
	upgrader   websocket.Upgrader
}

// NewHandler 创建 WebSocket 处理器。
func NewHandler(devices *device.Service, dispatcher *dispatch.Service, workflows *workflow.Service) *Handler {
	return &Handler{
		devices:    devices,
		dispatcher: dispatcher,
		workflows:  workflows,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
	}
}

// ServeHTTP 升级为 WebSocket 并处理设备消息。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade websocket", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	var currentDeviceID string
	var deviceConn *dispatch.DeviceConn

	defer func() {
		if currentDeviceID == "" {
			return
		}
		h.dispatcher.UnregisterDeviceConn(currentDeviceID, conn)
		if err := h.devices.MarkOffline(context.Background(), currentDeviceID, time.Now()); err != nil {
			log.Printf("mark offline for %s: %v", currentDeviceID, err)
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var msg protocol.Envelope
		if err := json.Unmarshal(data, &msg); err != nil {
			if deviceConn == nil {
				deviceConn = dispatch.NewDeviceConn(conn)
			}
			_ = writeAck(deviceConn, "invalid_message", "", "invalid_json")
			continue
		}

		switch msg.Type {
		case protocol.MessageTypeHello:
			if msg.DeviceID == "" {
				if deviceConn == nil {
					deviceConn = dispatch.NewDeviceConn(conn)
				}
				_ = writeAck(deviceConn, protocol.MessageTypeHello, msg.RequestID, "missing_device_id")
				continue
			}

			currentDeviceID = msg.DeviceID
			deviceConn = h.dispatcher.RegisterDeviceConn(currentDeviceID, conn)
			becameOnline, err := h.devices.MarkOnline(context.Background(), currentDeviceID, time.Now())
			if err != nil {
				_ = writeAck(deviceConn, protocol.MessageTypeHello, msg.RequestID, "server_error")
				continue
			}
			if profile, ok := parseExecutionProfile(msg.Payload); ok {
				if err := h.devices.UpdateExecutionProfile(context.Background(), currentDeviceID, profile); err != nil {
					log.Printf("update execution profile for %s via %s: %v", currentDeviceID, protocol.MessageTypeHello, err)
				}
			}
			if becameOnline {
				log.Printf("device %s became online via %s", currentDeviceID, protocol.MessageTypeHello)
			}
			_ = writeAck(deviceConn, protocol.MessageTypeHello, msg.RequestID, "ok")

		case protocol.MessageTypeHeartbeat:
			if msg.DeviceID == "" {
				if deviceConn == nil {
					deviceConn = dispatch.NewDeviceConn(conn)
				}
				_ = writeAck(deviceConn, protocol.MessageTypeHeartbeat, msg.RequestID, "missing_device_id")
				continue
			}

			currentDeviceID = msg.DeviceID
			deviceConn = h.dispatcher.RegisterDeviceConn(currentDeviceID, conn)
			becameOnline, err := h.devices.MarkOnline(context.Background(), currentDeviceID, time.Now())
			if err != nil {
				_ = writeAck(deviceConn, protocol.MessageTypeHeartbeat, msg.RequestID, "server_error")
				continue
			}
			if profile, ok := parseExecutionProfile(msg.Payload); ok {
				if err := h.devices.UpdateExecutionProfile(context.Background(), currentDeviceID, profile); err != nil {
					log.Printf("update execution profile for %s via %s: %v", currentDeviceID, protocol.MessageTypeHeartbeat, err)
				}
			}
			if becameOnline {
				log.Printf("device %s became online via %s", currentDeviceID, protocol.MessageTypeHeartbeat)
			}
			_ = writeAck(deviceConn, protocol.MessageTypeHeartbeat, msg.RequestID, "ok")

		case protocol.MessageTypeTaskAck:
			currentDeviceID = msg.DeviceID
			deviceConn = h.dispatcher.RegisterDeviceConn(currentDeviceID, conn)
			taskItem, err := h.dispatcher.HandleTaskAck(context.Background(), msg)
			if err != nil {
				log.Printf("handle task_ack for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeTaskAck, msg.RequestID, "server_error")
				continue
			}
			if _, err := h.dispatcher.MarkTaskRunning(context.Background(), taskItem.TaskID, msg.RequestID, "", "设备已确认并开始执行任务"); err != nil {
				log.Printf("mark task running for device %s: %v", currentDeviceID, err)
			}
			log.Printf("device %s acknowledged task %s", currentDeviceID, taskItem.TaskID)
			_ = writeAck(deviceConn, protocol.MessageTypeTaskAck, msg.RequestID, "ok")

		case protocol.MessageTypeWorkflowSessionAck:
			currentDeviceID = msg.DeviceID
			deviceConn = h.dispatcher.RegisterDeviceConn(currentDeviceID, conn)
			if h.workflows == nil {
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionAck, msg.RequestID, "server_error")
				continue
			}

			payloadBytes, err := json.Marshal(msg.Payload)
			if err != nil {
				log.Printf("marshal workflow_session_ack for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionAck, msg.RequestID, "server_error")
				continue
			}

			var payload protocol.WorkflowSessionAckPayload
			if err := json.Unmarshal(payloadBytes, &payload); err != nil {
				log.Printf("decode workflow_session_ack for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionAck, msg.RequestID, "server_error")
				continue
			}

			if err := h.workflows.HandleSessionAck(context.Background(), payload, msg.RequestID, currentDeviceID); err != nil {
				log.Printf("handle workflow_session_ack for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionAck, msg.RequestID, "server_error")
				continue
			}
			log.Printf("device %s acknowledged workflow session %s", currentDeviceID, payload.WorkflowRunID)
			_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionAck, msg.RequestID, "ok")

		case protocol.MessageTypeWorkflowSessionEvent:
			currentDeviceID = msg.DeviceID
			deviceConn = h.dispatcher.RegisterDeviceConn(currentDeviceID, conn)
			if h.workflows == nil {
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionEvent, msg.RequestID, "server_error")
				continue
			}

			payloadBytes, err := json.Marshal(msg.Payload)
			if err != nil {
				log.Printf("marshal workflow_session_event for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionEvent, msg.RequestID, "server_error")
				continue
			}

			var payload protocol.WorkflowSessionEventPayload
			if err := json.Unmarshal(payloadBytes, &payload); err != nil {
				log.Printf("decode workflow_session_event for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionEvent, msg.RequestID, "server_error")
				continue
			}

			if err := h.workflows.HandleSessionEvent(context.Background(), payload, msg.RequestID, currentDeviceID); err != nil {
				log.Printf("handle workflow_session_event for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionEvent, msg.RequestID, "server_error")
				continue
			}
			log.Printf("device %s reported workflow session event run=%s event=%s", currentDeviceID, payload.WorkflowRunID, payload.EventType)
			_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionEvent, msg.RequestID, "ok")

		case protocol.MessageTypeWorkflowSessionResult:
			currentDeviceID = msg.DeviceID
			deviceConn = h.dispatcher.RegisterDeviceConn(currentDeviceID, conn)
			if h.workflows == nil {
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionResult, msg.RequestID, "server_error")
				continue
			}

			payloadBytes, err := json.Marshal(msg.Payload)
			if err != nil {
				log.Printf("marshal workflow_session_result for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionResult, msg.RequestID, "server_error")
				continue
			}

			var payload protocol.WorkflowSessionResultPayload
			if err := json.Unmarshal(payloadBytes, &payload); err != nil {
				log.Printf("decode workflow_session_result for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionResult, msg.RequestID, "server_error")
				continue
			}

			if err := h.workflows.HandleSessionResult(context.Background(), payload, msg.RequestID, currentDeviceID); err != nil {
				log.Printf("handle workflow_session_result for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionResult, msg.RequestID, "server_error")
				continue
			}
			log.Printf("device %s reported workflow session result %s -> %s", currentDeviceID, payload.WorkflowRunID, payload.Status)
			_ = writeAck(deviceConn, protocol.MessageTypeWorkflowSessionResult, msg.RequestID, "ok")

		case protocol.MessageTypeTaskProgress:
			currentDeviceID = msg.DeviceID
			deviceConn = h.dispatcher.RegisterDeviceConn(currentDeviceID, conn)
			taskItem, err := h.dispatcher.HandleTaskProgress(context.Background(), msg)
			if err != nil {
				log.Printf("handle task_progress for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeTaskProgress, msg.RequestID, "server_error")
				continue
			}
			log.Printf("device %s reported task progress %s -> %s", currentDeviceID, taskItem.TaskID, taskItem.CurrentStep)
			_ = writeAck(deviceConn, protocol.MessageTypeTaskProgress, msg.RequestID, "ok")

		case protocol.MessageTypeWorkflowStepProgress:
			currentDeviceID = msg.DeviceID
			deviceConn = h.dispatcher.RegisterDeviceConn(currentDeviceID, conn)
			if h.workflows == nil {
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowStepProgress, msg.RequestID, "server_error")
				continue
			}

			payloadBytes, err := json.Marshal(msg.Payload)
			if err != nil {
				log.Printf("marshal workflow_step_progress for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowStepProgress, msg.RequestID, "server_error")
				continue
			}

			var payload workflow.StepProgressPayload
			if err := json.Unmarshal(payloadBytes, &payload); err != nil {
				log.Printf("decode workflow_step_progress for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowStepProgress, msg.RequestID, "server_error")
				continue
			}

			if err := h.workflows.HandleStepProgress(context.Background(), payload, msg.RequestID, currentDeviceID); err != nil {
				log.Printf("handle workflow_step_progress for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeWorkflowStepProgress, msg.RequestID, "server_error")
				continue
			}
			log.Printf("device %s reported workflow step progress run=%s node=%s step=%s", currentDeviceID, payload.WorkflowRunID, payload.WorkflowNodeID, payload.StepName)
			_ = writeAck(deviceConn, protocol.MessageTypeWorkflowStepProgress, msg.RequestID, "ok")

		case protocol.MessageTypeTaskResult:
			currentDeviceID = msg.DeviceID
			deviceConn = h.dispatcher.RegisterDeviceConn(currentDeviceID, conn)
			taskItem, err := h.dispatcher.HandleTaskResult(context.Background(), msg)
			if err != nil {
				log.Printf("handle task_result for device %s: %v", currentDeviceID, err)
				_ = writeAck(deviceConn, protocol.MessageTypeTaskResult, msg.RequestID, "server_error")
				continue
			}
			log.Printf("device %s reported task result %s -> %s", currentDeviceID, taskItem.TaskID, taskItem.Status)
			_ = writeAck(deviceConn, protocol.MessageTypeTaskResult, msg.RequestID, "ok")

		default:
			if deviceConn == nil {
				deviceConn = dispatch.NewDeviceConn(conn)
			}
			_ = writeAck(deviceConn, msg.Type, msg.RequestID, "unsupported_type")
		}
	}
}

func parseExecutionProfile(payload any) (device.ExecutionProfile, bool) {
	payloadMap, ok := payload.(map[string]any)
	if !ok || payloadMap == nil {
		return device.ExecutionProfile{}, false
	}

	rawProfile, ok := payloadMap["execution_profile"]
	if !ok || rawProfile == nil {
		return device.ExecutionProfile{}, false
	}

	profileMap, ok := rawProfile.(map[string]any)
	if !ok || profileMap == nil {
		return device.ExecutionProfile{}, false
	}

	return device.ExecutionProfile{
		AccessibilityStatus:              stringifyProfileValue(profileMap["accessibility_status"]),
		ForegroundServiceStatus:          stringifyProfileValue(profileMap["foreground_service_status"]),
		BatteryOptimizationIgnoredStatus: stringifyProfileValue(profileMap["battery_optimization_ignored_status"]),
		CheckedAt:                        stringifyProfileValue(profileMap["checked_at"]),
		Message:                          stringifyProfileValue(profileMap["message"]),
	}, true
}

func stringifyProfileValue(value any) string {
	return strings.TrimSpace(strings.ReplaceAll(toString(value), "\x00", ""))
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

// writeAck 向客户端返回最小确认报文。
func writeAck(conn *dispatch.DeviceConn, messageType string, requestID string, status string) error {
	resp := protocol.Envelope{
		Type:      "ack",
		RequestID: requestID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"message_type": messageType,
			"status":       status,
		},
	}

	return conn.WriteJSON(resp)
}
