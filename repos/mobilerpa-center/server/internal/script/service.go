package script

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	// ErrScriptNameRequired 表示缺少脚本名称。
	ErrScriptNameRequired = errors.New("script_name is required")
	// ErrScriptVersionRequired 表示缺少脚本版本。
	ErrScriptVersionRequired = errors.New("script_version is required")
	// ErrScriptVersionNotFound 表示目标脚本版本不存在。
	ErrScriptVersionNotFound = errors.New("script version not found")
	// ErrScriptNotFound 表示指定脚本名称不存在。
	ErrScriptNotFound = errors.New("script not found")
	// ErrScriptPathUnsafe 表示脚本名称、版本或入口文件包含不安全路径片段。
	ErrScriptPathUnsafe = errors.New("script path is unsafe")
	// ErrScriptSourceTypeUnsupported 表示上传来源类型不支持。
	ErrScriptSourceTypeUnsupported = errors.New("script source_type is unsupported")
	// ErrScriptVersionAlreadyExists 表示脚本版本已存在。
	ErrScriptVersionAlreadyExists = errors.New("script version already exists")
	// ErrScriptEntryNotFound 表示版本根目录下找不到 index.js。
	ErrScriptEntryNotFound = errors.New("cannot find index.js in zip or directory")
	// ErrScriptUploadEmpty 表示上传内容为空。
	ErrScriptUploadEmpty = errors.New("script upload is empty")
	// ErrScriptRepositoryUnavailable 表示当前服务没有可用的脚本版本仓库存储。
	ErrScriptRepositoryUnavailable = errors.New("script repository is unavailable")
	ErrScriptVersionReferenced    = errors.New("script version is referenced by workflows")
	ErrScriptReferenced           = errors.New("script is referenced by workflows")
)

const (
	// SourceTypeZip 表示脚本来源为 zip 上传。
	SourceTypeZip = "zip"
	// StorageTypeDirectory 表示中心内部统一以目录形式保存脚本版本。
	StorageTypeDirectory = "directory"
)

// FileMeta 描述脚本目录中的单个文件。
type FileMeta struct {
	// RelativePath 是相对于脚本版本目录的相对路径。
	RelativePath string `json:"relative_path"`
	// ChecksumSHA256 是文件的 SHA-256 摘要。
	ChecksumSHA256 string `json:"checksum_sha256"`
}

// Manifest 描述 Agent 执行脚本前需要确认的版本元数据。
type Manifest struct {
	// ScriptName 是脚本名称。
	ScriptName string `json:"script_name"`
	// ScriptVersion 是脚本版本。
	ScriptVersion string `json:"script_version"`
	// EntryFile 是脚本包内入口文件。
	EntryFile string `json:"entry_file"`
	// ChecksumSHA256 是入口文件的 SHA-256 摘要。
	ChecksumSHA256 string `json:"checksum_sha256"`
	// DownloadURL 是脚本文件下载接口。
	DownloadURL string `json:"download_url"`
	// Files 是脚本版本目录内的完整文件清单。
	Files []FileMeta `json:"files"`
	// SourceType 是原始上传来源类型。
	SourceType string `json:"source_type"`
	// StorageType 是中心内部存储类型。
	StorageType string `json:"storage_type"`
}

// File 描述可下载的脚本文件。
type File struct {
	// Manifest 是脚本版本元数据。
	Manifest Manifest
	// Path 是本地真实文件路径。
	Path string
}

// VersionSummary 描述脚本版本摘要。
type VersionSummary struct {
	// ScriptName 是脚本名称。
	ScriptName string `json:"script_name"`
	// ScriptVersion 是脚本版本。
	ScriptVersion string `json:"script_version"`
	// EntryFile 是入口文件。
	EntryFile string `json:"entry_file"`
	// SourceType 是上传来源类型。
	SourceType string `json:"source_type"`
	// StorageType 是中心存储类型。
	StorageType string `json:"storage_type"`
	// Status 是版本状态。
	Status string `json:"status"`
	// CreatedAt 是创建时间。
	CreatedAt string `json:"created_at"`
	WorkflowReferences []WorkflowReference `json:"workflow_references"`
}

// ScriptSummary 描述脚本及其版本列表。
type ScriptSummary struct {
	// ScriptName 是脚本名称。
	ScriptName string `json:"script_name"`
	// Versions 是该脚本已维护的版本列表。
	Versions []VersionSummary `json:"versions"`
}

type WorkflowReference struct {
	WorkflowDefID string `json:"workflow_def_id"`
	WorkflowName  string `json:"workflow_name"`
	NodeID        string `json:"node_id"`
	NodeName      string `json:"node_name"`
}

type ReferenceConflictError struct {
	ScriptName    string
	ScriptVersion string
	References    []WorkflowReference

	cause error
}

func (e *ReferenceConflictError) Error() string {
	if e == nil || e.cause == nil {
		return "script reference conflict"
	}
	return e.cause.Error()
}

func (e *ReferenceConflictError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// UploadRequest 描述脚本上传入库请求。
type UploadRequest struct {
	// ScriptName 是脚本名称。
	ScriptName string
	// ScriptVersion 是脚本版本。
	ScriptVersion string
	// SourceType 是来源类型，当前支持 zip。
	SourceType string
	// Force 表示当脚本版本已存在时是否强制覆盖。
	Force bool
	// FileHeader 是上传文件头。
	FileHeader *multipart.FileHeader
}

// UploadResult 描述脚本上传入库结果。
type UploadResult struct {
	// ScriptName 是脚本名称。
	ScriptName string `json:"script_name"`
	// ScriptVersion 是脚本版本。
	ScriptVersion string `json:"script_version"`
	// EntryFile 是入口文件。
	EntryFile string `json:"entry_file"`
	// SourceType 是原始来源类型。
	SourceType string `json:"source_type"`
	// StorageType 是中心存储类型。
	StorageType string `json:"storage_type"`
	// StoredPath 是中心脚本库存储目录。
	StoredPath string `json:"stored_path"`
}

// Service 提供脚本版本上传、查询与下载能力。
type Service struct {
	db   *sql.DB
	root string
}

// NewService 创建脚本服务。
func NewService(db *sql.DB, root string) *Service {
	return &Service{
		db:   db,
		root: root,
	}
}

// ListScripts 返回当前中心脚本库中所有脚本及其版本列表。
func (s *Service) ListScripts(ctx context.Context) ([]ScriptSummary, error) {
	if s.db == nil {
		return nil, ErrScriptRepositoryUnavailable
	}
	referencesByVersion, err := s.listWorkflowReferences(ctx, "", "")
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT script_name, version, entry_file, source_type, storage_type, status, created_at
FROM script_versions
ORDER BY script_name ASC, version ASC`)
	if err != nil {
		return nil, fmt.Errorf("query script versions: %w", err)
	}
	defer rows.Close()

	grouped := make([]ScriptSummary, 0)
	indexByName := make(map[string]int)

	for rows.Next() {
		var item VersionSummary
		if err := rows.Scan(
			&item.ScriptName,
			&item.ScriptVersion,
			&item.EntryFile,
			&item.SourceType,
			&item.StorageType,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan script version: %w", err)
		}
		item.WorkflowReferences = append(item.WorkflowReferences, referencesByVersion[scriptVersionKey(item.ScriptName, item.ScriptVersion)]...)

		groupIndex, ok := indexByName[item.ScriptName]
		if !ok {
			grouped = append(grouped, ScriptSummary{
				ScriptName: item.ScriptName,
				Versions:   []VersionSummary{},
			})
			groupIndex = len(grouped) - 1
			indexByName[item.ScriptName] = groupIndex
		}

		grouped[groupIndex].Versions = append(grouped[groupIndex].Versions, item)
	}

	return grouped, rows.Err()
}

// GetManifest 返回指定脚本版本的元数据。
func (s *Service) GetManifest(ctx context.Context, scriptName string, scriptVersion string) (Manifest, error) {
	file, err := s.GetFile(ctx, scriptName, scriptVersion, "index.js")
	if err != nil {
		return Manifest{}, err
	}
	return file.Manifest, nil
}

// GetFile 返回指定脚本版本中的指定文件。
func (s *Service) GetFile(ctx context.Context, scriptName string, scriptVersion string, relativePath string) (File, error) {
	scriptName = strings.TrimSpace(scriptName)
	scriptVersion = strings.TrimSpace(scriptVersion)
	relativePath = strings.TrimSpace(relativePath)

	if scriptName == "" {
		return File{}, ErrScriptNameRequired
	}
	if scriptVersion == "" {
		return File{}, ErrScriptVersionRequired
	}
	if !isSafePathPart(scriptName) || !isSafePathPart(scriptVersion) {
		return File{}, ErrScriptPathUnsafe
	}

	if relativePath == "" {
		relativePath = "index.js"
	}
	relativePath = filepath.ToSlash(relativePath)
	if !isSafeRelativePath(relativePath) {
		return File{}, ErrScriptPathUnsafe
	}

	versionInfo, err := s.findVersion(ctx, scriptName, scriptVersion)
	if err != nil {
		return File{}, err
	}

	versionDir := versionInfo.FilePath
	path := filepath.Join(versionDir, filepath.FromSlash(relativePath))
	cleanRoot, err := filepath.Abs(s.root)
	if err != nil {
		return File{}, fmt.Errorf("resolve script root: %w", err)
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return File{}, fmt.Errorf("resolve script path: %w", err)
	}
	if !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) && cleanPath != cleanRoot {
		return File{}, ErrScriptPathUnsafe
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, ErrScriptVersionNotFound
		}
		return File{}, fmt.Errorf("stat script file: %w", err)
	}
	if info.IsDir() {
		return File{}, ErrScriptVersionNotFound
	}

	checksum, err := checksumFile(cleanPath)
	if err != nil {
		return File{}, err
	}

	files, err := listFiles(versionDir)
	if err != nil {
		return File{}, err
	}

	return File{
		Path: cleanPath,
		Manifest: Manifest{
			ScriptName:     scriptName,
			ScriptVersion:  scriptVersion,
			EntryFile:      versionInfo.EntryFile,
			ChecksumSHA256: checksum,
			DownloadURL:    "/api/v1/script/download?script_name=" + scriptName + "&script_version=" + scriptVersion + "&relative_path=" + versionInfo.EntryFile,
			Files:          files,
			SourceType:     versionInfo.SourceType,
			StorageType:    versionInfo.StorageType,
		},
	}, nil
}

// UploadZip 把 zip 文件上传到中心脚本库，并统一解压为版本目录。
func (s *Service) UploadZip(ctx context.Context, req UploadRequest) (UploadResult, error) {
	if s.db == nil {
		return UploadResult{}, ErrScriptRepositoryUnavailable
	}
	req.ScriptName = strings.TrimSpace(req.ScriptName)
	req.ScriptVersion = strings.TrimSpace(req.ScriptVersion)
	req.SourceType = strings.TrimSpace(req.SourceType)

	if req.ScriptName == "" {
		return UploadResult{}, ErrScriptNameRequired
	}
	if req.ScriptVersion == "" {
		return UploadResult{}, ErrScriptVersionRequired
	}
	if !isSafePathPart(req.ScriptName) || !isSafePathPart(req.ScriptVersion) {
		return UploadResult{}, ErrScriptPathUnsafe
	}
	if req.SourceType == "" {
		req.SourceType = SourceTypeZip
	}
	if req.SourceType != SourceTypeZip {
		return UploadResult{}, ErrScriptSourceTypeUnsupported
	}
	if req.FileHeader == nil || req.FileHeader.Size == 0 {
		return UploadResult{}, ErrScriptUploadEmpty
	}

	if _, err := s.findVersion(ctx, req.ScriptName, req.ScriptVersion); err == nil {
		if !req.Force {
			return UploadResult{}, ErrScriptVersionAlreadyExists
		}
	} else if !errors.Is(err, ErrScriptVersionNotFound) {
		return UploadResult{}, err
	}

	versionDir := filepath.Join(s.root, req.ScriptName, req.ScriptVersion)
	tempDir := versionDir + ".uploading"
	if err := os.RemoveAll(tempDir); err != nil {
		return UploadResult{}, fmt.Errorf("remove temp script dir: %w", err)
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return UploadResult{}, fmt.Errorf("mkdir temp script dir: %w", err)
	}

	file, err := req.FileHeader.Open()
	if err != nil {
		return UploadResult{}, fmt.Errorf("open upload file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return UploadResult{}, fmt.Errorf("read upload file: %w", err)
	}

	if err := unzipToDir(data, tempDir); err != nil {
		_ = os.RemoveAll(tempDir)
		if errors.Is(err, ErrScriptEntryNotFound) {
			return UploadResult{}, err
		}
		return UploadResult{}, fmt.Errorf("unzip script package: %w", err)
	}

	entryPath := filepath.Join(tempDir, "index.js")
	if _, err := os.Stat(entryPath); err != nil {
		_ = os.RemoveAll(tempDir)
		return UploadResult{}, ErrScriptEntryNotFound
	}

	checksum, err := checksumFile(entryPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return UploadResult{}, err
	}

	if err := os.RemoveAll(versionDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return UploadResult{}, fmt.Errorf("remove old script dir: %w", err)
	}
	if err := os.Rename(tempDir, versionDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return UploadResult{}, fmt.Errorf("move script dir: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if req.Force {
		if _, err := s.db.ExecContext(ctx, `
DELETE FROM script_versions
WHERE script_name = ? AND version = ?`,
			req.ScriptName,
			req.ScriptVersion,
		); err != nil {
			return UploadResult{}, fmt.Errorf("delete old script version: %w", err)
		}
	}

	if _, err := s.db.ExecContext(ctx, `
INSERT INTO script_versions (
    script_name, version, entry_file, checksum, file_path, storage_type, source_type, status, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ScriptName,
		req.ScriptVersion,
		"index.js",
		checksum,
		versionDir,
		StorageTypeDirectory,
		req.SourceType,
		"dev",
		now,
	); err != nil {
		return UploadResult{}, fmt.Errorf("insert script version: %w", err)
	}

	return UploadResult{
		ScriptName:    req.ScriptName,
		ScriptVersion: req.ScriptVersion,
		EntryFile:     "index.js",
		SourceType:    req.SourceType,
		StorageType:   StorageTypeDirectory,
		StoredPath:    versionDir,
	}, nil
}

// DeleteVersion 删除指定脚本版本的数据库记录和本地目录。
func (s *Service) DeleteVersion(ctx context.Context, scriptName string, scriptVersion string) error {
	scriptName = strings.TrimSpace(scriptName)
	scriptVersion = strings.TrimSpace(scriptVersion)

	if scriptName == "" {
		return ErrScriptNameRequired
	}
	if scriptVersion == "" {
		return ErrScriptVersionRequired
	}
	if !isSafePathPart(scriptName) || !isSafePathPart(scriptVersion) {
		return ErrScriptPathUnsafe
	}

	versionInfo, err := s.findVersion(ctx, scriptName, scriptVersion)
	if err != nil {
		return err
	}
	if err := s.ensureVersionNotReferenced(ctx, scriptName, scriptVersion); err != nil {
		return err
	}

	if s.db != nil {
		result, err := s.db.ExecContext(ctx, `
DELETE FROM script_versions
WHERE script_name = ? AND version = ?`,
			scriptName,
			scriptVersion,
		)
		if err != nil {
			return fmt.Errorf("delete script version: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("rows affected for delete script version: %w", err)
		}
		if affected == 0 {
			return ErrScriptVersionNotFound
		}
	}

	if err := os.RemoveAll(versionInfo.FilePath); err != nil {
		return fmt.Errorf("remove script version dir: %w", err)
	}

	return nil
}

// DeleteScript 删除指定脚本名称下的全部版本和脚本根目录。
func (s *Service) DeleteScript(ctx context.Context, scriptName string) error {
	scriptName = strings.TrimSpace(scriptName)

	if scriptName == "" {
		return ErrScriptNameRequired
	}
	if !isSafePathPart(scriptName) {
		return ErrScriptPathUnsafe
	}

	scriptDir := filepath.Join(s.root, scriptName)
	if err := s.ensureScriptNotReferenced(ctx, scriptName); err != nil {
		return err
	}

	if s.db != nil {
		row := s.db.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM script_versions
WHERE script_name = ?`,
			scriptName,
		)

		var count int
		if err := row.Scan(&count); err != nil {
			return fmt.Errorf("count script versions: %w", err)
		}
		if count == 0 {
			return ErrScriptNotFound
		}

		if _, err := s.db.ExecContext(ctx, `
DELETE FROM script_versions
WHERE script_name = ?`,
			scriptName,
		); err != nil {
			return fmt.Errorf("delete script: %w", err)
		}
	} else {
		info, err := os.Stat(scriptDir)
		if err != nil {
			if os.IsNotExist(err) {
				return ErrScriptNotFound
			}
			return fmt.Errorf("stat script dir: %w", err)
		}
		if !info.IsDir() {
			return ErrScriptNotFound
		}
	}

	if err := os.RemoveAll(scriptDir); err != nil {
		return fmt.Errorf("remove script dir: %w", err)
	}

	return nil
}

type versionRecord struct {
	ScriptName   string
	ScriptVersion string
	EntryFile    string
	Checksum     string
	FilePath     string
	StorageType  string
	SourceType   string
	Status       string
	CreatedAt    string
}

func (s *Service) findVersion(ctx context.Context, scriptName string, scriptVersion string) (versionRecord, error) {
	if s.db == nil {
		versionDir := filepath.Join(s.root, scriptName, scriptVersion)
		entryPath := filepath.Join(versionDir, "index.js")
		if _, err := os.Stat(entryPath); err != nil {
			if os.IsNotExist(err) {
				return versionRecord{}, ErrScriptVersionNotFound
			}
			return versionRecord{}, fmt.Errorf("stat script version: %w", err)
		}
		checksum, err := checksumFile(entryPath)
		if err != nil {
			return versionRecord{}, err
		}
		return versionRecord{
			ScriptName:    scriptName,
			ScriptVersion: scriptVersion,
			EntryFile:     "index.js",
			Checksum:      checksum,
			FilePath:      versionDir,
			StorageType:   StorageTypeDirectory,
			SourceType:    "",
			Status:        "dev",
			CreatedAt:     "",
		}, nil
	}

	row := s.db.QueryRowContext(ctx, `
SELECT script_name, version, entry_file, checksum, file_path, storage_type, source_type, status, created_at
FROM script_versions
WHERE script_name = ? AND version = ?`,
		scriptName,
		scriptVersion,
	)

	var item versionRecord
	if err := row.Scan(
		&item.ScriptName,
		&item.ScriptVersion,
		&item.EntryFile,
		&item.Checksum,
		&item.FilePath,
		&item.StorageType,
		&item.SourceType,
		&item.Status,
		&item.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return versionRecord{}, ErrScriptVersionNotFound
		}
		return versionRecord{}, fmt.Errorf("query script version: %w", err)
	}

	return item, nil
}

func isSafePathPart(value string) bool {
	if value == "" {
		return false
	}
	if strings.Contains(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, "..") {
		return false
	}
	return true
}

func isSafeRelativePath(value string) bool {
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "\\") {
		return false
	}
	if strings.Contains(value, "..") {
		return false
	}
	for _, item := range strings.Split(value, "/") {
		item = strings.TrimSpace(item)
		if item == "" || item == "." || item == ".." {
			return false
		}
	}
	return true
}

func listFiles(versionDir string) ([]FileMeta, error) {
	result := make([]FileMeta, 0, 8)

	err := filepath.Walk(versionDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(versionDir, path)
		if err != nil {
			return fmt.Errorf("rel script file: %w", err)
		}
		relativePath = filepath.ToSlash(relativePath)
		if !isSafeRelativePath(relativePath) {
			return ErrScriptPathUnsafe
		}

		checksum, err := checksumFile(path)
		if err != nil {
			return err
		}

		result = append(result, FileMeta{
			RelativePath:   relativePath,
			ChecksumSHA256: checksum,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk script files: %w", err)
	}

	sort.Slice(result, func(i int, j int) bool {
		return result[i].RelativePath < result[j].RelativePath
	})

	return result, nil
}

func unzipToDir(data []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	rootPrefix := detectZipRootPrefix(reader.File)
	foundEntry := false
	for _, file := range reader.File {
		name := filepath.ToSlash(strings.TrimSpace(file.Name))
		if name == "" {
			continue
		}

		trimmed := trimZipRoot(name, rootPrefix)
		if trimmed == "" {
			continue
		}
		if !isSafeRelativePath(trimmed) {
			return ErrScriptPathUnsafe
		}
		if trimmed == "index.js" {
			foundEntry = true
		}

		targetPath := filepath.Join(targetDir, filepath.FromSlash(trimmed))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("mkdir unzip dir: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("mkdir unzip parent: %w", err)
		}

		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip file: %w", err)
		}

		output, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("create unzip file: %w", err)
		}

		if _, err := io.Copy(output, rc); err != nil {
			output.Close()
			rc.Close()
			return fmt.Errorf("copy unzip file: %w", err)
		}

		output.Close()
		rc.Close()
	}

	if !foundEntry {
		return ErrScriptEntryNotFound
	}

	return nil
}

func detectZipRootPrefix(files []*zip.File) string {
	if len(files) == 0 {
		return ""
	}

	rootName := ""
	for _, file := range files {
		name := filepath.ToSlash(strings.TrimSpace(file.Name))
		if name == "" {
			continue
		}

		parts := splitZipPath(name)
		if len(parts) <= 1 {
			return ""
		}

		if rootName == "" {
			rootName = parts[0]
			continue
		}
		if parts[0] != rootName {
			return ""
		}
	}

	if rootName == "" {
		return ""
	}
	return rootName + "/"
}

func trimZipRoot(name string, rootPrefix string) string {
	name = strings.TrimSpace(filepath.ToSlash(name))
	if rootPrefix != "" && strings.HasPrefix(name, rootPrefix) {
		name = strings.TrimPrefix(name, rootPrefix)
	}

	parts := splitZipPath(name)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "/")
}

func splitZipPath(name string) []string {
	parts := strings.Split(name, "/")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

func checksumFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open script file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash script file: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func scriptVersionKey(scriptName string, scriptVersion string) string {
	return strings.TrimSpace(scriptName) + "@" + strings.TrimSpace(scriptVersion)
}

func (s *Service) ensureVersionNotReferenced(ctx context.Context, scriptName string, scriptVersion string) error {
	referencesByVersion, err := s.listWorkflowReferences(ctx, scriptName, scriptVersion)
	if err != nil {
		return err
	}
	references := referencesByVersion[scriptVersionKey(scriptName, scriptVersion)]
	if len(references) == 0 {
		return nil
	}
	return &ReferenceConflictError{
		ScriptName:    scriptName,
		ScriptVersion: scriptVersion,
		References:    references,
		cause:         ErrScriptVersionReferenced,
	}
}

func (s *Service) ensureScriptNotReferenced(ctx context.Context, scriptName string) error {
	referencesByVersion, err := s.listWorkflowReferences(ctx, scriptName, "")
	if err != nil {
		return err
	}
	references := make([]WorkflowReference, 0)
	for _, items := range referencesByVersion {
		references = append(references, items...)
	}
	if len(references) == 0 {
		return nil
	}
	return &ReferenceConflictError{
		ScriptName: scriptName,
		References: references,
		cause:      ErrScriptReferenced,
	}
}

func (s *Service) listWorkflowReferences(ctx context.Context, scriptName string, scriptVersion string) (map[string][]WorkflowReference, error) {
	result := make(map[string][]WorkflowReference)
	if s.db == nil {
		return result, nil
	}

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`
SELECT n.script_name, n.script_version, n.workflow_def_id, d.workflow_name, n.node_id, n.node_name
FROM workflow_nodes n
JOIN workflow_defs d
  ON d.id = n.workflow_def_id
WHERE n.node_type = 'script'`)

	args := make([]any, 0, 2)
	if strings.TrimSpace(scriptName) != "" {
		queryBuilder.WriteString(" AND n.script_name = ?")
		args = append(args, strings.TrimSpace(scriptName))
	}
	if strings.TrimSpace(scriptVersion) != "" {
		queryBuilder.WriteString(" AND n.script_version = ?")
		args = append(args, strings.TrimSpace(scriptVersion))
	}
	queryBuilder.WriteString(`
ORDER BY n.script_name ASC, n.script_version ASC, n.workflow_def_id ASC, n.position ASC`)

	rows, err := s.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query script workflow references: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ref WorkflowReference
		var refScriptName string
		var refScriptVersion string
		if err := rows.Scan(
			&refScriptName,
			&refScriptVersion,
			&ref.WorkflowDefID,
			&ref.WorkflowName,
			&ref.NodeID,
			&ref.NodeName,
		); err != nil {
			return nil, fmt.Errorf("scan script workflow reference: %w", err)
		}
		key := scriptVersionKey(refScriptName, refScriptVersion)
		result[key] = append(result[key], ref)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate script workflow references: %w", err)
	}

	return result, nil
}
