// Package warmup provides warmup operations for NOVA deployment.
// This includes model downloading and image pre-pulling to optimize startup time.
package warmup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/host/nfs"
)

// ModelDownloadResult contains the result of a model download operation.
type ModelDownloadResult struct {
	// ModelPath is the absolute path to the downloaded model on the host
	ModelPath string

	// Success indicates whether the download completed successfully
	Success bool

	// Error contains any error that occurred during download
	Error error
}

// downloader manages a background model download operation.
type downloader struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	cfg        *config.Config
	modelPath  string
	result     *ModelDownloadResult
	done       chan struct{}
	mu         sync.Mutex
}

// StartModelDownloadAsync starts downloading a model in the background.
// The download runs in an Alpine container using Python and the huggingface_hub package.
// This approach avoids permission issues with the standalone installer script.
//
// If the download fails, it cancels the provided context to stop the parent deployment process.
//
// Returns a function that can be called to wait for download completion.
// Returns nil if no model is configured (download skipped).
func StartModelDownloadAsync(ctx context.Context, cancelFunc context.CancelFunc, cfg *config.Config) func() (*ModelDownloadResult, error) {
	if cfg.LLM.Model == "" {
		ui.Debug("No LLM model configured, skipping download")
		return nil
	}

	d := &downloader{
		ctx:        ctx,
		cancelFunc: cancelFunc,
		cfg:        cfg,
		result:     &ModelDownloadResult{},
		done:       make(chan struct{}),
	}

	// Prepare model directory
	// Download to ~/.nova/share/nfs/models/{model-slug}/ for multi-model caching
	// This path will be accessible via NFS as /nfs-export/models/{model-slug}
	modelSlug := cfg.GetModelSlug()
	modelsPath, err := nfs.GetModelsPath(cfg)
	if err != nil {
		ui.Warn("Failed to get models path: %v", err)
		return nil
	}

	d.modelPath = filepath.Join(modelsPath, modelSlug)

	// Check if model already exists and is complete
	if isModelComplete(d.modelPath) {
		ui.Info("Model already cached: %s", cfg.LLM.Model)
		d.result.ModelPath = d.modelPath
		d.result.Success = true
		close(d.done)
		return d.wait
	}

	// Start download in background goroutine
	go d.download()

	return d.wait
}

// download performs the actual model download in a background goroutine.
func (d *downloader) download() {
	defer close(d.done)

	ui.Info("Starting model download in background: %s", d.cfg.LLM.Model)

	// Ensure parent directory exists
	if err := os.MkdirAll(d.modelPath, 0755); err != nil {
		d.setResult(nil, fmt.Errorf("failed to create model directory: %w", err))
		return
	}

	// Build download command using Python and huggingface_hub package
	// Run as current user with tmpfs for /tmp to avoid permission issues
	uid := os.Getuid()
	gid := os.Getgid()

	args := []string{
		"run",
		"--rm",
		"--user", fmt.Sprintf("%d:%d", uid, gid),
		"-v", fmt.Sprintf("%s:/models", d.modelPath),
		// Use tmpfs for /tmp - Docker-managed, no host permission issues
		"--tmpfs", "/tmp:rw,exec,mode=1777",
		"-w", "/tmp",
		"-e", "HOME=/tmp",
		"-e", "TMPDIR=/tmp",
	}

	// Add HF token as environment variable if provided
	if d.cfg.LLM.HfToken != "" {
		args = append(args, "-e", fmt.Sprintf("HF_TOKEN=%s", d.cfg.LLM.HfToken))
	}

	// Build the download command using uv tool runner
	downloadCmd := "pip install uv --no-cache-dir && "
	downloadCmd += "python -m uv tool run hf download " + d.cfg.LLM.Model + " --local-dir /models"
	if d.cfg.LLM.HfToken != "" {
		downloadCmd += " --token $HF_TOKEN"
	}

	// Use Python Alpine image with uv tool runner
	args = append(args, []string{
		"python:3.14-alpine",
		"sh", "-c",
		downloadCmd,
	}...)

	// Run download command
	cmd := exec.CommandContext(d.ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		d.setResult(nil, fmt.Errorf("download failed: %w\nOutput: %s", err, string(output)))
		return
	}

	// Get model size for reporting
	size := getDirectorySize(d.modelPath)

	ui.Success("Model downloaded successfully (%s): %s", size, d.cfg.LLM.Model)

	d.setResult(&ModelDownloadResult{
		ModelPath: d.modelPath,
		Success:   true,
	}, nil)
}

// wait blocks until the download completes and returns the result.
func (d *downloader) wait() (*ModelDownloadResult, error) {
	<-d.done

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.result.Error != nil {
		return d.result, d.result.Error
	}

	return d.result, nil
}

// setResult sets the download result in a thread-safe manner.
// If an error occurred, it cancels the parent context to stop deployment immediately.
func (d *downloader) setResult(result *ModelDownloadResult, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if result != nil {
		d.result = result
	}

	if err != nil {
		d.result.Error = err
		d.result.Success = false
		ui.Error("Model download failed: %v", err)
		ui.Error("Cancelling deployment - warmup is required for tier 3")
		// Cancel parent context to stop deployment immediately
		d.cancelFunc()
	}
}

// isModelComplete checks if a model directory exists and contains files.
func isModelComplete(modelPath string) bool {
	info, err := os.Stat(modelPath)
	if err != nil {
		return false
	}

	if !info.IsDir() {
		return false
	}

	// Check if directory has any files
	entries, err := os.ReadDir(modelPath)
	if err != nil || len(entries) == 0 {
		return false
	}

	// Directory exists and has content
	return true
}

// getDirectorySize returns a human-readable size of a directory.
func getDirectorySize(path string) string {
	cmd := exec.Command("du", "-sh", path)
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	parts := strings.Fields(string(output))
	if len(parts) > 0 {
		return parts[0]
	}

	return "unknown"
}
