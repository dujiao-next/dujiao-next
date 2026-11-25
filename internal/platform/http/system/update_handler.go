package systemhttp

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/selfupdate"
	"github.com/dujiao-next/internal/version"

	"github.com/gin-gonic/gin"
)

// GetUpdateCapability 返回当前部署环境是否支持一键升级。
// 前端据此决定展示升级按钮还是手动升级指引（容器部署走 compose 命令）。
// GET /api/v1/admin/system/update/capability
func (h *AdminHandler) GetUpdateCapability(c *gin.Context) {
	capability := selfupdate.Detect()
	var sourceUI gin.H
	if h.sourceUpdate != nil {
		capability.CanUpdate = false
		capability.HasBackup = false
		plan, _ := h.sourceUpdate.cachedPlan()
		sourceUI = gin.H{"base": h.sourceUpdate.base, "plan": plan}
	}
	response.Success(c, gin.H{
		"capability":    capability,
		"state":         h.updates.Snapshot(),
		"source_update": sourceUI,
	})
}

// StartUpdate 启动一键升级任务。任务在后台执行，进度由 GetUpdateStatus 轮询。
// POST /api/v1/admin/system/update/start
func (h *AdminHandler) StartUpdate(c *gin.Context) {
	if h.sourceUpdate != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_not_supported", nil)
		return
	}
	err := h.updates.Start(c.Request.Context())
	switch {
	case err == nil:
		state := h.updates.Snapshot()
		// 替换二进制是不可逆的高危操作，留下操作者与目标版本便于事后追溯
		logOperation(c, "self_update_started", "target_version", state.TargetVersion)
		response.Success(c, state)
	case errors.Is(err, selfupdate.ErrNotSupported):
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_not_supported", err)
	case errors.Is(err, selfupdate.ErrUpdateInProgress):
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_in_progress", err)
	case errors.Is(err, selfupdate.ErrNoUpdateAvailable):
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_already_latest", err)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.update_failed", err)
	}
}

// GetUpdateStatus 轮询升级任务进度
// GET /api/v1/admin/system/update/status
func (h *AdminHandler) GetUpdateStatus(c *gin.Context) {
	response.Success(c, h.updates.Snapshot())
}

// rollbackRequest 回滚请求体。force 用于在迁移已开始或元数据不可信时确认风险后强制回滚。
type rollbackRequest struct {
	Force bool `json:"force"`
}

// RollbackUpdate 还原到升级前的二进制。
//
// 只有「二进制已替换但新版本还没开始迁移，且升级元数据可信」这个窗口内回滚是安全的。
// AutoMigrate 一旦开始，即使后来失败也可能已部分推进 schema，退回旧二进制未必兼容。
// 这种情况下默认拒绝，返回 error.update_rollback_unsafe，让前端确认后带 force 重试。
//
// 注意：如果新版本压根起不来，这个 HTTP 接口本身也是不可用的。那种场景请在终端执行
// `dujiao-next rollback`，它不依赖 HTTP 服务与数据库。
// POST /api/v1/admin/system/update/rollback
func (h *AdminHandler) RollbackUpdate(c *gin.Context) {
	if h.sourceUpdate != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_not_supported", nil)
		return
	}
	var req rollbackRequest
	// 允许空 body：老前端与 curl 调用不带 JSON 时按 force=false 处理
	_ = c.ShouldBindJSON(&req)

	err := h.updates.Rollback(req.Force)
	switch {
	case err == nil:
		logOperation(c, "self_update_rolled_back", "forced", req.Force)
		response.Success(c, h.updates.Snapshot())
	case errors.Is(err, selfupdate.ErrNoBackup):
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_no_backup", err)
	case errors.Is(err, selfupdate.ErrRollbackUnsafe):
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_rollback_unsafe", err)
	case errors.Is(err, selfupdate.ErrUpdateInProgress):
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_in_progress", err)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.update_rollback_failed", err)
	}
}

// RestartService 让进程退出，由 systemd 拉起替换后的新二进制。
// 没有守护进程时拒绝执行 —— 那种情况下退出等于停服。
// POST /api/v1/admin/system/restart
func (h *AdminHandler) RestartService(c *gin.Context) {
	if h.updates.Running() {
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_in_progress", selfupdate.ErrUpdateInProgress)
		return
	}
	if !selfupdate.Detect().CanRestart {
		ginutil.RespondError(c, response.CodeBadRequest, "error.restart_not_supported", selfupdate.ErrNotSupported)
		return
	}

	logOperation(c, "self_update_restart_requested")

	// 先应答再退出：Restart 内部延迟半秒发 SIGTERM，
	// 保证这条响应能写回客户端，前端才知道该进入等待重连状态。
	response.Success(c, gin.H{"restarting": true})
	selfupdate.Restart()
}

// logOperation 记录升级类操作的操作者。这类接口会替换程序文件或重启进程，
// 出问题时需要能回答「谁在什么时候动了什么版本」。
func logOperation(c *gin.Context, action string, kv ...any) {
	adminID, _ := ginutil.GetAdminID(c)
	fields := append([]any{"admin_id", adminID, "client_ip", c.ClientIP()}, kv...)
	ginutil.RequestLog(c).Infow(action, fields...)
}

type sourceUpdate struct {
	base, snapshot, script, packagePath string
	snapshotHash                        string
	mu                                  sync.Mutex
}

type sourcePlan struct {
	Base          string         `json:"base"`
	Target        string         `json:"target"`
	Summary       map[string]int `json:"summary"`
	Conflicts     []string       `json:"conflicts"`
	ReadyForBuild bool           `json:"ready_for_build"`
	SnapshotHash  string         `json:"snapshot_sha256"`
}

func configuredSourceUpdate() *sourceUpdate {
	paths := &sourceUpdate{
		base:        os.Getenv("DUJIAO_SOURCE_UPDATE_BASE"),
		snapshot:    os.Getenv("DUJIAO_SOURCE_UPDATE_SNAPSHOT"),
		script:      os.Getenv("DUJIAO_SOURCE_UPDATE_SCRIPT"),
		packagePath: os.Getenv("DUJIAO_SOURCE_UPDATE_PACKAGE"),
	}
	if paths.base == "" || paths.snapshot == "" || paths.script == "" || paths.packagePath == "" {
		return nil
	}
	if file, err := os.Open(paths.snapshot); err == nil {
		hash := sha256.New()
		if size, err := io.Copy(hash, io.LimitReader(file, (32<<20)+1)); err == nil && size <= 32<<20 {
			paths.snapshotHash = fmt.Sprintf("%x", hash.Sum(nil))
		}
		_ = file.Close()
	}
	return paths
}

func (s *sourceUpdate) cachedPlan() (*sourcePlan, error) {
	archive, err := zip.OpenReader(s.packagePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	for _, entry := range archive.File {
		if entry.Name != "plan.json" {
			continue
		}
		if entry.UncompressedSize64 > 8<<20 {
			break
		}
		file, err := entry.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(file, 8<<20))
		_ = file.Close()
		if err != nil {
			return nil, err
		}
		var plan sourcePlan
		if err := json.Unmarshal(data, &plan); err != nil {
			return nil, err
		}
		if plan.Base != s.base || s.snapshotHash == "" || plan.SnapshotHash != s.snapshotHash {
			return nil, os.ErrNotExist
		}
		return &plan, nil
	}
	return nil, errors.New("source update package has no valid manifest")
}

func (h *AdminHandler) PrepareSourceUpdate(c *gin.Context) {
	source := h.sourceUpdate
	if source == nil || source.snapshotHash == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_not_supported", nil)
		return
	}
	if !source.mu.TryLock() {
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_in_progress", nil)
		return
	}
	defer source.mu.Unlock()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 52*time.Second)
	defer cancel()
	release, err := h.releases.CheckLatestRelease(ctx)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.update_check_failed", err)
		return
	}
	newer, err := version.IsNewerVersion(release.LatestVersion, source.base)
	if err != nil || !newer {
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_already_latest", err)
		return
	}

	command := exec.CommandContext(ctx, "/usr/bin/python3", "-I", source.script,
		source.base, release.LatestVersion, source.snapshot, source.packagePath)
	if output, err := command.CombinedOutput(); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.update_source_failed",
			fmt.Errorf("source update: %w: %s", err, output))
		return
	}
	plan, err := source.cachedPlan()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.update_source_failed", err)
		return
	}
	logOperation(c, "source_update_prepared", "base_version", source.base, "target_version", release.LatestVersion)
	response.Success(c, plan)
}

func (h *AdminHandler) DownloadSourceUpdate(c *gin.Context) {
	if h.sourceUpdate == nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.update_not_supported", nil)
		return
	}
	if _, err := h.sourceUpdate.cachedPlan(); err != nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.update_source_missing", err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.FileAttachment(h.sourceUpdate.packagePath, "dujiao-source-increment.zip")
}
