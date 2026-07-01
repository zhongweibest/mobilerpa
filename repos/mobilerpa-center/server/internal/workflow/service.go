package workflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
)

const (
	DefinitionStatusDraft  = "draft"
	DefinitionStatusActive = "active"

	NodeTypeScript = "script"
	NodeTypeLoop   = "loop"
	NodeTypeStop   = "stop"

	EdgeTypeNext     = "next"
	EdgeTypeLoopBody = "loop_body"
	EdgeTypeLoopExit = "loop_exit"
)

var (
	ErrWorkflowDefinitionNotFound      = errors.New("workflow definition not found")
	ErrWorkflowDefinitionNameRequired  = errors.New("workflow_name is required")
	ErrWorkflowDefinitionNodesRequired = errors.New("workflow nodes are required")
	ErrWorkflowNodeIDRequired          = errors.New("workflow node_id is required")
	ErrWorkflowNodeTypeUnsupported     = errors.New("workflow node_type is unsupported")
	ErrWorkflowScriptNameRequired      = errors.New("workflow script_name is required")
)

// Definition 表示工作流定义。
type Definition struct {
	WorkflowDefID      string `json:"workflow_def_id"`
	WorkflowName       string `json:"workflow_name"`
	Description        string `json:"description"`
	BuilderSegmentsJSON string `json:"builder_segments_json"`
	Status             string `json:"status"`
	Nodes              []Node `json:"nodes"`
	Edges              []Edge `json:"edges"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// Node 表示工作流节点定义。
type Node struct {
	WorkflowDefID string `json:"workflow_def_id"`
	NodeID        string `json:"node_id"`
	NodeType      string `json:"node_type"`
	NodeName      string `json:"node_name"`
	ScriptName    string `json:"script_name"`
	ScriptVersion string `json:"script_version"`
	MaxIterations int    `json:"max_iterations"`
	Position      int    `json:"position"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Edge 表示工作流节点之间的连线关系。
type Edge struct {
	WorkflowDefID string `json:"workflow_def_id"`
	FromNodeID    string `json:"from_node_id"`
	ToNodeID      string `json:"to_node_id"`
	EdgeType      string `json:"edge_type"`
	CreatedAt     string `json:"created_at"`
}

// CreateDefinitionRequest 描述创建工作流定义的请求。
type CreateDefinitionRequest struct {
	WorkflowName        string `json:"workflow_name"`
	Description         string `json:"description"`
	BuilderSegmentsJSON string `json:"builder_segments_json"`
	Status              string `json:"status"`
	Nodes               []Node `json:"nodes"`
	Edges               []Edge `json:"edges"`
}

// UpdateDefinitionRequest 描述更新工作流定义的请求。
type UpdateDefinitionRequest struct {
	WorkflowName        string `json:"workflow_name"`
	Description         string `json:"description"`
	BuilderSegmentsJSON string `json:"builder_segments_json"`
	Status              string `json:"status"`
	Nodes               []Node `json:"nodes"`
	Edges               []Edge `json:"edges"`
}

// Service 现在只负责工作流定义与快照读取，不再承载运行实例生命周期。
type Service struct {
	db      *sql.DB
	devices *device.Service
	tasks   *task.Service
}

// NewService 创建工作流服务。
func NewService(db *sql.DB, devices *device.Service, tasks *task.Service, _ any) *Service {
	return &Service{
		db:      db,
		devices: devices,
		tasks:   tasks,
	}
}

// CreateDefinition 创建新的工作流定义。
func (s *Service) CreateDefinition(ctx context.Context, req CreateDefinitionRequest) (Definition, error) {
	req.WorkflowName = strings.TrimSpace(req.WorkflowName)
	req.Description = strings.TrimSpace(req.Description)
	req.BuilderSegmentsJSON = strings.TrimSpace(req.BuilderSegmentsJSON)
	req.Status = strings.TrimSpace(req.Status)
	if req.WorkflowName == "" {
		return Definition{}, ErrWorkflowDefinitionNameRequired
	}
	if len(req.Nodes) == 0 {
		return Definition{}, ErrWorkflowDefinitionNodesRequired
	}
	if req.Status == "" {
		req.Status = DefinitionStatusDraft
	}

	for index := range req.Nodes {
		req.Nodes[index].NodeID = strings.TrimSpace(req.Nodes[index].NodeID)
		req.Nodes[index].NodeType = strings.TrimSpace(req.Nodes[index].NodeType)
		req.Nodes[index].NodeName = strings.TrimSpace(req.Nodes[index].NodeName)
		req.Nodes[index].ScriptName = strings.TrimSpace(req.Nodes[index].ScriptName)
		req.Nodes[index].ScriptVersion = strings.TrimSpace(req.Nodes[index].ScriptVersion)
		if req.Nodes[index].NodeID == "" {
			return Definition{}, ErrWorkflowNodeIDRequired
		}
		switch req.Nodes[index].NodeType {
		case NodeTypeScript:
			if req.Nodes[index].ScriptName == "" {
				return Definition{}, ErrWorkflowScriptNameRequired
			}
		case NodeTypeLoop, NodeTypeStop:
		default:
			return Definition{}, ErrWorkflowNodeTypeUnsupported
		}
		req.Nodes[index].Position = index + 1
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Definition{}, fmt.Errorf("begin workflow definition tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(ctx, `
INSERT INTO workflow_defs (
    workflow_name, description, builder_segments_json, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		req.WorkflowName,
		req.Description,
		req.BuilderSegmentsJSON,
		req.Status,
		now,
		now,
	)
	if err != nil {
		return Definition{}, fmt.Errorf("insert workflow definition: %w", err)
	}

	insertedID, err := result.LastInsertId()
	if err != nil {
		return Definition{}, fmt.Errorf("read inserted workflow definition id: %w", err)
	}
	workflowDefID := strconv.FormatInt(insertedID, 10)

	for _, node := range req.Nodes {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO workflow_nodes (
    workflow_def_id, node_id, node_type, node_name, script_name, script_version,
    max_iterations, position, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			workflowDefID,
			node.NodeID,
			node.NodeType,
			node.NodeName,
			node.ScriptName,
			node.ScriptVersion,
			node.MaxIterations,
			node.Position,
			now,
			now,
		); err != nil {
			return Definition{}, fmt.Errorf("insert workflow node %s: %w", node.NodeID, err)
		}
	}

	for _, edge := range req.Edges {
		edge.FromNodeID = strings.TrimSpace(edge.FromNodeID)
		edge.ToNodeID = strings.TrimSpace(edge.ToNodeID)
		edge.EdgeType = strings.TrimSpace(edge.EdgeType)
		if edge.EdgeType == "" {
			edge.EdgeType = EdgeTypeNext
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO workflow_edges (
    workflow_def_id, from_node_id, to_node_id, edge_type, created_at
) VALUES (?, ?, ?, ?, ?)`,
			workflowDefID,
			edge.FromNodeID,
			edge.ToNodeID,
			edge.EdgeType,
			now,
		); err != nil {
			return Definition{}, fmt.Errorf("insert workflow edge %s -> %s: %w", edge.FromNodeID, edge.ToNodeID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Definition{}, fmt.Errorf("commit workflow definition tx: %w", err)
	}
	tx = nil

	return s.GetDefinition(ctx, workflowDefID)
}

// UpdateDefinition 更新已有工作流定义及其节点、边。
func (s *Service) UpdateDefinition(ctx context.Context, workflowDefID string, req UpdateDefinitionRequest) (Definition, error) {
	workflowDefID = strings.TrimSpace(workflowDefID)
	if workflowDefID == "" {
		return Definition{}, ErrWorkflowDefinitionNotFound
	}
	if _, err := s.GetDefinition(ctx, workflowDefID); err != nil {
		return Definition{}, err
	}

	req.WorkflowName = strings.TrimSpace(req.WorkflowName)
	req.Description = strings.TrimSpace(req.Description)
	req.BuilderSegmentsJSON = strings.TrimSpace(req.BuilderSegmentsJSON)
	req.Status = strings.TrimSpace(req.Status)
	if req.WorkflowName == "" {
		return Definition{}, ErrWorkflowDefinitionNameRequired
	}
	if len(req.Nodes) == 0 {
		return Definition{}, ErrWorkflowDefinitionNodesRequired
	}
	if req.Status == "" {
		req.Status = DefinitionStatusDraft
	}

	for index := range req.Nodes {
		req.Nodes[index].NodeID = strings.TrimSpace(req.Nodes[index].NodeID)
		req.Nodes[index].NodeType = strings.TrimSpace(req.Nodes[index].NodeType)
		req.Nodes[index].NodeName = strings.TrimSpace(req.Nodes[index].NodeName)
		req.Nodes[index].ScriptName = strings.TrimSpace(req.Nodes[index].ScriptName)
		req.Nodes[index].ScriptVersion = strings.TrimSpace(req.Nodes[index].ScriptVersion)
		if req.Nodes[index].NodeID == "" {
			return Definition{}, ErrWorkflowNodeIDRequired
		}
		switch req.Nodes[index].NodeType {
		case NodeTypeScript:
			if req.Nodes[index].ScriptName == "" {
				return Definition{}, ErrWorkflowScriptNameRequired
			}
		case NodeTypeLoop, NodeTypeStop:
		default:
			return Definition{}, ErrWorkflowNodeTypeUnsupported
		}
		req.Nodes[index].Position = index + 1
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Definition{}, fmt.Errorf("begin workflow update tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
UPDATE workflow_defs
SET workflow_name = ?, description = ?, builder_segments_json = ?, status = ?, updated_at = ?
WHERE id = ?`,
		req.WorkflowName,
		req.Description,
		req.BuilderSegmentsJSON,
		req.Status,
		now,
		workflowDefID,
	); err != nil {
		return Definition{}, fmt.Errorf("update workflow definition: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_edges
WHERE workflow_def_id = ?`, workflowDefID); err != nil {
		return Definition{}, fmt.Errorf("delete existing workflow edges: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_nodes
WHERE workflow_def_id = ?`, workflowDefID); err != nil {
		return Definition{}, fmt.Errorf("delete existing workflow nodes: %w", err)
	}

	for _, node := range req.Nodes {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO workflow_nodes (
    workflow_def_id, node_id, node_type, node_name, script_name, script_version,
    max_iterations, position, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			workflowDefID,
			node.NodeID,
			node.NodeType,
			node.NodeName,
			node.ScriptName,
			node.ScriptVersion,
			node.MaxIterations,
			node.Position,
			now,
			now,
		); err != nil {
			return Definition{}, fmt.Errorf("insert updated workflow node %s: %w", node.NodeID, err)
		}
	}

	for _, edge := range req.Edges {
		edge.FromNodeID = strings.TrimSpace(edge.FromNodeID)
		edge.ToNodeID = strings.TrimSpace(edge.ToNodeID)
		edge.EdgeType = strings.TrimSpace(edge.EdgeType)
		if edge.EdgeType == "" {
			edge.EdgeType = EdgeTypeNext
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO workflow_edges (
    workflow_def_id, from_node_id, to_node_id, edge_type, created_at
) VALUES (?, ?, ?, ?, ?)`,
			workflowDefID,
			edge.FromNodeID,
			edge.ToNodeID,
			edge.EdgeType,
			now,
		); err != nil {
			return Definition{}, fmt.Errorf("insert updated workflow edge %s -> %s: %w", edge.FromNodeID, edge.ToNodeID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Definition{}, fmt.Errorf("commit workflow update tx: %w", err)
	}
	tx = nil

	return s.GetDefinition(ctx, workflowDefID)
}

// ListDefinitions 返回工作流定义列表。
func (s *Service) ListDefinitions(ctx context.Context) ([]Definition, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS workflow_def_id, workflow_name, description, builder_segments_json, status, created_at, updated_at
FROM workflow_defs
ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query workflow definitions: %w", err)
	}
	defer rows.Close()

	items := make([]Definition, 0)
	for rows.Next() {
		var item Definition
		if err := rows.Scan(
			&item.WorkflowDefID,
			&item.WorkflowName,
			&item.Description,
			&item.BuilderSegmentsJSON,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workflow definition: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow definitions: %w", err)
	}

	if len(items) == 0 {
		return items, nil
	}

	workflowDefIDs := make([]string, 0, len(items))
	for _, item := range items {
		workflowDefIDs = append(workflowDefIDs, item.WorkflowDefID)
	}

	nodesByDefinition, err := s.listNodesByDefinitions(ctx, workflowDefIDs)
	if err != nil {
		return nil, fmt.Errorf("list workflow definition nodes: %w", err)
	}
	edgesByDefinition, err := s.listEdgesByDefinitions(ctx, workflowDefIDs)
	if err != nil {
		return nil, fmt.Errorf("list workflow definition edges: %w", err)
	}

	for index := range items {
		items[index].Nodes = nodesByDefinition[items[index].WorkflowDefID]
		items[index].Edges = edgesByDefinition[items[index].WorkflowDefID]
	}

	return items, nil
}

// GetDefinition 返回单个工作流定义及其节点、边。
func (s *Service) GetDefinition(ctx context.Context, workflowDefID string) (Definition, error) {
	workflowDefID = strings.TrimSpace(workflowDefID)
	if workflowDefID == "" {
		return Definition{}, ErrWorkflowDefinitionNotFound
	}

	var item Definition
	row := s.db.QueryRowContext(ctx, `
SELECT id AS workflow_def_id, workflow_name, description, builder_segments_json, status, created_at, updated_at
FROM workflow_defs
WHERE id = ?`,
		workflowDefID,
	)
	if err := row.Scan(
		&item.WorkflowDefID,
		&item.WorkflowName,
		&item.Description,
		&item.BuilderSegmentsJSON,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Definition{}, ErrWorkflowDefinitionNotFound
		}
		return Definition{}, fmt.Errorf("get workflow definition: %w", err)
	}

	nodes, err := s.listNodes(ctx, workflowDefID)
	if err != nil {
		return Definition{}, err
	}
	edges, err := s.listEdges(ctx, workflowDefID)
	if err != nil {
		return Definition{}, err
	}

	item.Nodes = nodes
	item.Edges = edges
	return item, nil
}

// DeleteDefinition 删除工作流定义及其节点、边。
func (s *Service) DeleteDefinition(ctx context.Context, workflowDefID string) error {
	workflowDefID = strings.TrimSpace(workflowDefID)
	if workflowDefID == "" {
		return ErrWorkflowDefinitionNotFound
	}

	if _, err := s.GetDefinition(ctx, workflowDefID); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow delete tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_edges
WHERE workflow_def_id = ?`, workflowDefID); err != nil {
		return fmt.Errorf("delete workflow edges: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_nodes
WHERE workflow_def_id = ?`, workflowDefID); err != nil {
		return fmt.Errorf("delete workflow nodes: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
DELETE FROM workflow_defs
WHERE id = ?`, workflowDefID)
	if err != nil {
		return fmt.Errorf("delete workflow definition: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("workflow definition rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrWorkflowDefinitionNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workflow delete tx: %w", err)
	}
	tx = nil
	return nil
}

func (s *Service) listNodes(ctx context.Context, workflowDefID string) ([]Node, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT workflow_def_id, node_id, node_type, node_name, script_name, script_version,
       max_iterations, position, created_at, updated_at
FROM workflow_nodes
WHERE workflow_def_id = ?
ORDER BY position ASC, id ASC`, workflowDefID)
	if err != nil {
		return nil, fmt.Errorf("query workflow nodes: %w", err)
	}
	defer rows.Close()

	items := make([]Node, 0)
	for rows.Next() {
		item, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow nodes: %w", err)
	}
	return items, nil
}

func (s *Service) listNodesByDefinitions(ctx context.Context, workflowDefIDs []string) (map[string][]Node, error) {
	result := make(map[string][]Node, len(workflowDefIDs))
	for _, workflowDefID := range workflowDefIDs {
		result[workflowDefID] = []Node{}
	}
	if len(workflowDefIDs) == 0 {
		return result, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(workflowDefIDs)), ",")
	args := make([]any, 0, len(workflowDefIDs))
	for _, workflowDefID := range workflowDefIDs {
		args = append(args, workflowDefID)
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
SELECT workflow_def_id, node_id, node_type, node_name, script_name, script_version,
       max_iterations, position, created_at, updated_at
FROM workflow_nodes
WHERE workflow_def_id IN (%s)
ORDER BY workflow_def_id ASC, position ASC, id ASC`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("query workflow nodes by definitions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		result[item.WorkflowDefID] = append(result[item.WorkflowDefID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow nodes by definitions: %w", err)
	}
	return result, nil
}

func (s *Service) listEdges(ctx context.Context, workflowDefID string) ([]Edge, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT workflow_def_id, from_node_id, to_node_id, edge_type, created_at
FROM workflow_edges
WHERE workflow_def_id = ?
ORDER BY id ASC`, workflowDefID)
	if err != nil {
		return nil, fmt.Errorf("query workflow edges: %w", err)
	}
	defer rows.Close()

	items := make([]Edge, 0)
	for rows.Next() {
		item, err := scanEdge(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow edges: %w", err)
	}
	return items, nil
}

func (s *Service) listEdgesByDefinitions(ctx context.Context, workflowDefIDs []string) (map[string][]Edge, error) {
	result := make(map[string][]Edge, len(workflowDefIDs))
	for _, workflowDefID := range workflowDefIDs {
		result[workflowDefID] = []Edge{}
	}
	if len(workflowDefIDs) == 0 {
		return result, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(workflowDefIDs)), ",")
	args := make([]any, 0, len(workflowDefIDs))
	for _, workflowDefID := range workflowDefIDs {
		args = append(args, workflowDefID)
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
SELECT workflow_def_id, from_node_id, to_node_id, edge_type, created_at
FROM workflow_edges
WHERE workflow_def_id IN (%s)
ORDER BY workflow_def_id ASC, id ASC`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("query workflow edges by definitions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item, err := scanEdge(rows)
		if err != nil {
			return nil, err
		}
		result[item.WorkflowDefID] = append(result[item.WorkflowDefID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow edges by definitions: %w", err)
	}
	return result, nil
}

type nodeScanner interface {
	Scan(dest ...any) error
}

func scanNode(scanner nodeScanner) (Node, error) {
	var item Node
	if err := scanner.Scan(
		&item.WorkflowDefID,
		&item.NodeID,
		&item.NodeType,
		&item.NodeName,
		&item.ScriptName,
		&item.ScriptVersion,
		&item.MaxIterations,
		&item.Position,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, ErrWorkflowDefinitionNotFound
		}
		return Node{}, fmt.Errorf("scan workflow node: %w", err)
	}
	return item, nil
}

type edgeScanner interface {
	Scan(dest ...any) error
}

func scanEdge(scanner edgeScanner) (Edge, error) {
	var item Edge
	if err := scanner.Scan(
		&item.WorkflowDefID,
		&item.FromNodeID,
		&item.ToNodeID,
		&item.EdgeType,
		&item.CreatedAt,
	); err != nil {
		return Edge{}, fmt.Errorf("scan workflow edge: %w", err)
	}
	return item, nil
}
