package software

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSoftwareNameRequired = errors.New("software_name_required")
	ErrSoftwareFileRequired = errors.New("software_file_required")
	ErrSoftwareNotFound     = errors.New("software_not_found")
)

type Package struct {
	// SoftwareID 是软件记录 ID。
	SoftwareID string `json:"software_id"`
	// SoftwareName 是软件名称。
	SoftwareName string `json:"software_name"`
	// Description 是软件描述。
	Description string `json:"description"`
	// PackageFileName 是上传包文件名。
	PackageFileName string `json:"package_file_name"`
	// PackageStoragePath 是中心本地存储路径。
	PackageStoragePath string `json:"package_storage_path"`
	// PackageSize 是软件包字节大小。
	PackageSize int64 `json:"package_size"`
	// CreatedAt 是记录创建时间。
	CreatedAt string `json:"created_at"`
	// UpdatedAt 是记录最后更新时间。
	UpdatedAt string `json:"updated_at"`
}

type CreateRequest struct {
	SoftwareName string
	Description  string
	FileHeader   *multipart.FileHeader
}

type UpdateRequest struct {
	SoftwareName string
	Description  string
	FileHeader   *multipart.FileHeader
}

type Service struct {
	db   *sql.DB
	root string
}

func NewService(db *sql.DB, root string) *Service {
	return &Service{
		db:   db,
		root: root,
	}
}

func (s *Service) List(ctx context.Context) ([]Package, error) {
	if s.db == nil {
		return []Package{}, nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, software_name, description, package_file_name, package_storage_path, package_size, created_at, updated_at
FROM software_packages
ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query software packages: %w", err)
	}
	defer rows.Close()

	result := make([]Package, 0)
	for rows.Next() {
		var item Package
		var id int64
		if err := rows.Scan(
			&id,
			&item.SoftwareName,
			&item.Description,
			&item.PackageFileName,
			&item.PackageStoragePath,
			&item.PackageSize,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan software package: %w", err)
		}
		item.SoftwareID = strconv.FormatInt(id, 10)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate software packages: %w", err)
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, softwareID string) (Package, error) {
	if s.db == nil {
		return Package{}, ErrSoftwareNotFound
	}

	id, err := parseID(softwareID)
	if err != nil {
		return Package{}, ErrSoftwareNotFound
	}

	var item Package
	var rawID int64
	err = s.db.QueryRowContext(ctx, `
SELECT id, software_name, description, package_file_name, package_storage_path, package_size, created_at, updated_at
FROM software_packages
WHERE id = ?`, id).Scan(
		&rawID,
		&item.SoftwareName,
		&item.Description,
		&item.PackageFileName,
		&item.PackageStoragePath,
		&item.PackageSize,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Package{}, ErrSoftwareNotFound
	}
	if err != nil {
		return Package{}, fmt.Errorf("query software package: %w", err)
	}
	item.SoftwareID = strconv.FormatInt(rawID, 10)
	return item, nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Package, error) {
	name := strings.TrimSpace(req.SoftwareName)
	if name == "" {
		return Package{}, ErrSoftwareNameRequired
	}
	if req.FileHeader == nil {
		return Package{}, ErrSoftwareFileRequired
	}
	if s.db == nil {
		return Package{}, errors.New("software_repository_unavailable")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	storedFileName, storedPath, fileSize, err := s.saveUploadFile(req.FileHeader)
	if err != nil {
		return Package{}, err
	}

	result, err := s.db.ExecContext(ctx, `
INSERT INTO software_packages (
    software_name, description, package_file_name, package_storage_path, package_size, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		name,
		strings.TrimSpace(req.Description),
		storedFileName,
		storedPath,
		fileSize,
		now,
		now,
	)
	if err != nil {
		_ = os.Remove(storedPath)
		return Package{}, fmt.Errorf("insert software package: %w", err)
	}

	insertID, err := result.LastInsertId()
	if err != nil {
		return Package{}, fmt.Errorf("get software insert id: %w", err)
	}

	return Package{
		SoftwareID:         strconv.FormatInt(insertID, 10),
		SoftwareName:       name,
		Description:        strings.TrimSpace(req.Description),
		PackageFileName:    storedFileName,
		PackageStoragePath: storedPath,
		PackageSize:        fileSize,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

func (s *Service) Update(ctx context.Context, softwareID string, req UpdateRequest) (Package, error) {
	current, err := s.Get(ctx, softwareID)
	if err != nil {
		return Package{}, err
	}

	name := strings.TrimSpace(req.SoftwareName)
	if name == "" {
		return Package{}, ErrSoftwareNameRequired
	}

	description := strings.TrimSpace(req.Description)
	fileName := current.PackageFileName
	storagePath := current.PackageStoragePath
	fileSize := current.PackageSize
	oldStoragePath := current.PackageStoragePath

	if req.FileHeader != nil {
		var saveErr error
		fileName, storagePath, fileSize, saveErr = s.saveUploadFile(req.FileHeader)
		if saveErr != nil {
			return Package{}, saveErr
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	id, _ := parseID(softwareID)
	_, err = s.db.ExecContext(ctx, `
UPDATE software_packages
SET software_name = ?, description = ?, package_file_name = ?, package_storage_path = ?, package_size = ?, updated_at = ?
WHERE id = ?`,
		name,
		description,
		fileName,
		storagePath,
		fileSize,
		now,
		id,
	)
	if err != nil {
		if req.FileHeader != nil {
			_ = os.Remove(storagePath)
		}
		return Package{}, fmt.Errorf("update software package: %w", err)
	}

	if req.FileHeader != nil && strings.TrimSpace(oldStoragePath) != "" && oldStoragePath != storagePath {
		_ = os.Remove(oldStoragePath)
	}

	return Package{
		SoftwareID:         softwareID,
		SoftwareName:       name,
		Description:        description,
		PackageFileName:    fileName,
		PackageStoragePath: storagePath,
		PackageSize:        fileSize,
		CreatedAt:          current.CreatedAt,
		UpdatedAt:          now,
	}, nil
}

func (s *Service) Delete(ctx context.Context, softwareID string) error {
	current, err := s.Get(ctx, softwareID)
	if err != nil {
		return err
	}

	id, _ := parseID(softwareID)
	result, err := s.db.ExecContext(ctx, `DELETE FROM software_packages WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete software package: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("software rows affected: %w", err)
	}
	if affected == 0 {
		return ErrSoftwareNotFound
	}

	if strings.TrimSpace(current.PackageStoragePath) != "" {
		_ = os.Remove(current.PackageStoragePath)
	}
	return nil
}

func (s *Service) saveUploadFile(fileHeader *multipart.FileHeader) (string, string, int64, error) {
	if fileHeader == nil {
		return "", "", 0, ErrSoftwareFileRequired
	}
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return "", "", 0, fmt.Errorf("mkdir software root: %w", err)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", "", 0, fmt.Errorf("open software upload file: %w", err)
	}
	defer src.Close()

	safeName := filepath.Base(fileHeader.Filename)
	if strings.TrimSpace(safeName) == "" {
		safeName = "package.bin"
	}

	storedFileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName)
	storedPath := filepath.Join(s.root, storedFileName)
	dst, err := os.Create(storedPath)
	if err != nil {
		return "", "", 0, fmt.Errorf("create software file: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		_ = os.Remove(storedPath)
		return "", "", 0, fmt.Errorf("save software file: %w", err)
	}

	return safeName, storedPath, written, nil
}

func parseID(raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid_id")
	}
	return value, nil
}
