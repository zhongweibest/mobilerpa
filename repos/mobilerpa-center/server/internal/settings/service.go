package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrSettingKeyRequired 表示配置项 key 不能为空。
	ErrSettingKeyRequired = errors.New("setting key is required")
)

// DiscoverySettings 描述设备发现页当前需要的系统配置。
type DiscoverySettings struct {
	// CenterBaseURL 是设备发现页默认使用的中心服务地址。
	CenterBaseURL string `json:"center_base_url"`
}

// Service 提供系统配置的读取与写入能力。
type Service struct {
	db *sql.DB
}

// NewService 创建系统配置服务。
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GetDiscoverySettings 返回设备发现页当前配置。
func (s *Service) GetDiscoverySettings(ctx context.Context) (DiscoverySettings, error) {
	return DiscoverySettings{
		CenterBaseURL: s.getValue(ctx, "discovery.center_base_url"),
	}, nil
}

// SaveDiscoverySettings 保存设备发现页配置。
func (s *Service) SaveDiscoverySettings(ctx context.Context, req DiscoverySettings) (DiscoverySettings, error) {
	if err := s.setValue(ctx, "discovery.center_base_url", req.CenterBaseURL); err != nil {
		return DiscoverySettings{}, err
	}
	return s.GetDiscoverySettings(ctx)
}

func (s *Service) getValue(ctx context.Context, key string) string {
	key = strings.TrimSpace(key)
	if key == "" || s.db == nil {
		return ""
	}

	row := s.db.QueryRowContext(ctx, `
SELECT setting_value
FROM system_settings
WHERE setting_key = ?`, key)

	var value string
	if err := row.Scan(&value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func (s *Service) setValue(ctx context.Context, key string, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return ErrSettingKeyRequired
	}
	if s.db == nil {
		return fmt.Errorf("settings storage unavailable")
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO system_settings (setting_key, setting_value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(setting_key) DO UPDATE SET
    setting_value = excluded.setting_value,
    updated_at = excluded.updated_at`,
		key,
		strings.TrimSpace(value),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save setting %s: %w", key, err)
	}
	return nil
}
