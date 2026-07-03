package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatalf("get cwd: %v", err)
	}

	tmpDocsDir := filepath.Join(root, "tmp_openapi_docs")
	swagJSONPath := filepath.Join(root, "tmp_openapi_swagger.json")
	outputPath := filepath.Join(root, "internal", "api", "generated", "openapi.json")
	defer cleanupTempArtifacts(tmpDocsDir, swagJSONPath)

	cmd := exec.Command("go", "run", "github.com/swaggo/swag/cmd/swag@v1.16.6",
		"init",
		"--generalInfo", "main.go",
		"--dir", strings.Join([]string{
			filepath.Join(root, "cmd", "center"),
			filepath.Join(root, "internal", "api"),
			filepath.Join(root, "internal", "device"),
			filepath.Join(root, "internal", "plan"),
			filepath.Join(root, "internal", "script"),
			filepath.Join(root, "internal", "software"),
			filepath.Join(root, "internal", "workflow"),
		}, ","),
		"--output", tmpDocsDir,
		"--outputTypes", "json",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("run swag init: %v", err)
	}

	generatedSwaggerJSON := filepath.Join(tmpDocsDir, "swagger.json")
	rawSwagger, err := os.ReadFile(generatedSwaggerJSON)
	if err != nil {
		log.Fatalf("read generated swagger json: %v", err)
	}
	if err := os.WriteFile(swagJSONPath, rawSwagger, 0o644); err != nil {
		log.Fatalf("write tmp swagger json: %v", err)
	}

	var swaggerDoc openapi2.T
	if err := json.Unmarshal(rawSwagger, &swaggerDoc); err != nil {
		log.Fatalf("decode swagger json: %v", err)
	}

	openAPIDoc, err := openapi2conv.ToV3(&swaggerDoc)
	if err != nil {
		log.Fatalf("convert swagger to openapi3: %v", err)
	}

	if err := openAPIDoc.Validate(openapi3.NewLoader().Context); err != nil {
		log.Fatalf("validate openapi3: %v", err)
	}

	result, err := json.MarshalIndent(openAPIDoc, "", "  ")
	if err != nil {
		log.Fatalf("marshal openapi3: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		log.Fatalf("mkdir output dir: %v", err)
	}
	if err := os.WriteFile(outputPath, result, 0o644); err != nil {
		log.Fatalf("write openapi3 json: %v", err)
	}
}

func cleanupTempArtifacts(tmpDocsDir string, tmpSwaggerPath string) {
	if err := os.RemoveAll(tmpDocsDir); err != nil && !os.IsNotExist(err) {
		log.Printf("cleanup tmp docs dir: %v", err)
	}
	if err := os.Remove(tmpSwaggerPath); err != nil && !os.IsNotExist(err) {
		log.Printf("cleanup tmp swagger json: %v", err)
	}
}
