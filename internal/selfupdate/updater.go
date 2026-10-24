package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dujiao-next/internal/version"
)

const (
	// maxArchiveSize 归档下载上限。内嵌两个 SPA 后二进制实测约 76MB（v1.3.1 之后），
	// tar.gz 压缩后远小于它，100MB 仍有充足余量，同时挡住异常大响应耗尽磁盘。
	maxArchiveSize = 100 << 20
	// maxBinarySize 解压后单个二进制的上限。归档经过压缩会明显小于二进制本身，
	// 所以这里必须比 maxArchiveSize 宽松，否则正常增长会先撞上解压上限。
	maxBinarySize = 200 << 20
	// maxChecksumSize checksums.txt 只有几行，给足冗余即可
	maxChecksumSize = 1 << 20
	// binaryName 归档内可执行文件名，与 .goreleaser.yaml 的 builds.binary 一致
	binaryName = "dujiao-next"

	downloadTimeout = 10 * time.Minute
	updateUserAgent = "dujiao-next-self-updater"
)

var (
	// ErrNoUpdateAvailable 当前已是最新版本
	ErrNoUpdateAvailable = errors.New("no update available")
	// ErrAssetNotFound 该发行版没有当前平台的归档
	ErrAssetNotFound = errors.New("no release asset for current platform")
	// ErrChecksumMismatch 下载内容与 checksums.txt 不符
	ErrChecksumMismatch = errors.New("checksum mismatch")
	// ErrNoBackup 没有可回滚的备份
	ErrNoBackup = errors.New("no backup available")
	// ErrRollbackUnsafe 数据库迁移已开始、或元数据不可信，回滚需显式确认
	ErrRollbackUnsafe = errors.New("database migration may have started; rollback may be incompatible with the current database")
	// ErrNotSupported 当前部署形态不支持一键升级
	ErrNotSupported = errors.New("self-update not supported in current environment")
)

// allowedDownloadHosts 限定下载源。GitHub 的 release 附件下载会从 api/github.com
// 302 到 objects.githubusercontent.com，两者都要放行，除此之外一律拒绝——
// 即便 GitHub API 返回被篡改，也不会把二进制换成任意第三方主机上的内容。
var allowedDownloadHosts = map[string]bool{
	"github.com":                           true,
	"objects.githubusercontent.com":        true,
	"release-assets.githubusercontent.com": true,
}

// Updater 执行下载、校验与二进制替换
type Updater struct {
	client *http.Client
}

// NewUpdater 创建 Updater。
// 不复用全局 http.DefaultClient：升级下载需要独立的长超时，
// 且必须自己接管重定向以逐跳校验主机白名单。
func NewUpdater() *Updater {
	return &Updater{
		client: &http.Client{
			Timeout: downloadTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("too many redirects")
				}
				return validateDownloadURL(req.URL)
			},
		},
	}
}

// applyLocked 执行实际升级，调用方必须已经持有 execPath 对应的跨进程锁。
//
// Manager.Start 在请求 GitHub release 之前取得锁并把所有权传到这里，从而覆盖
// 「版本检查 → 下载 → 校验 → 替换 → 元数据落盘」完整事务。
func (u *Updater) applyLocked(ctx context.Context, release *version.Release, execPath string, progress func(stage Stage, percent int)) error {
	report := func(stage Stage, percent int) {
		if progress != nil {
			progress(stage, percent)
		}
	}

	if release == nil {
		return errors.New("release is nil")
	}

	archiveURL, checksumURL, assetName, err := selectAssets(release.Assets)
	if err != nil {
		return err
	}

	// 临时目录必须与目标二进制同目录：os.Rename 只在同一文件系统内原子，
	// 放 /tmp 很可能跨挂载点（尤其容器与独立 /tmp 分区），rename 会退化成 EXDEV 失败。
	execDir := filepath.Dir(execPath)
	tempDir, err := os.MkdirTemp(execDir, ".dujiao-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	report(StageDownloading, 0)
	archivePath := filepath.Join(tempDir, assetName)
	if err := u.download(ctx, archiveURL, archivePath, maxArchiveSize, func(percent int) {
		report(StageDownloading, percent)
	}); err != nil {
		return fmt.Errorf("download archive: %w", err)
	}

	report(StageVerifying, 0)
	if checksumURL == "" {
		// checksums.txt 是 goreleaser 默认产物，缺失说明发行版不完整，
		// 与其跳过校验直接替换二进制，不如中止。
		return errors.New("checksums.txt not found in release assets")
	}
	if err := u.verifyChecksum(ctx, archivePath, assetName, checksumURL); err != nil {
		return err
	}

	report(StageExtracting, 0)
	newBinary := filepath.Join(tempDir, binaryName+".new")
	if err := extractBinary(archivePath, newBinary); err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}
	if err := os.Chmod(newBinary, 0o755); err != nil {
		return fmt.Errorf("chmod new binary: %w", err)
	}

	report(StageSwapping, 0)
	if err := swapBinary(execPath, newBinary); err != nil {
		return err
	}

	// 元数据写失败不让整个升级失败：二进制已经就位，回滚也只依赖 .backup 文件本身。
	// 丢掉的仅仅是「回滚会退到哪个版本、退回去安不安全」这部分提示信息。
	_ = writeBackupInfo(execPath, BackupInfo{
		PreviousVersion: version.Version,
		TargetVersion:   release.TagName,
		SwappedAt:       time.Now(),
	})

	return nil
}

// Rollback 用备份还原上一版本二进制。
//
// 只在「二进制已替换，但新版本尚未开始迁移，且升级元数据可信」的窗口内是安全的。
// AutoMigrate 一旦开始，即使随后失败，也可能已经部分推进 schema；退回旧二进制未必
// 读得懂库。因此这种情况下默认返回 ErrRollbackUnsafe，要求调用方显式 force。
func (u *Updater) Rollback(force bool) error {
	execPath, err := ExecutablePath()
	if err != nil {
		return err
	}
	return rollbackAt(execPath, force)
}

// rollbackAt 是 Rollback 的可测实现，execPath 由调用方给出而不是从进程自身解析。
// 拆出来是因为测试进程的可执行文件就是 go test 的临时二进制，没法在它旁边真的做替换。
func rollbackAt(execPath string, force bool) error {
	// 与 Apply 争同一把跨进程锁：后台升级和终端回滚是两个进程，只有这把锁能拦住它们交错
	unlock, err := acquireBinaryLock(execPath)
	if err != nil {
		return err
	}
	defer unlock()

	backup := backupPath(execPath)
	if _, err := os.Stat(backup); err != nil {
		return ErrNoBackup
	}

	info, known, err := ReadBackupInfo(execPath)
	if err != nil {
		return err
	}
	// 元数据不可信、迁移已经开始或新版本已经完整启动，任一命中都必须 fail-closed。
	// 尤其不能只看「完整启动」：AutoMigrate 可能已经部分修改 schema 后才报错。
	if rollbackUnsafe(info, known) && !force {
		return ErrRollbackUnsafe
	}

	// 在动二进制之前先落盘回滚标记。systemd 可能已经把新版本加载进另一个进程；
	// 即使那个进程此刻还没拿到启动锁，稍后也能看到标记并在迁移数据库前退出。
	supersededVersion := info.TargetVersion
	if strings.TrimSpace(supersededVersion) == "" {
		supersededVersion = version.Version
	}
	if err := writeRollbackMarker(execPath, supersededVersion); err != nil {
		return fmt.Errorf("prepare rollback marker: %w", err)
	}

	// 先把当前（新）二进制挪走，失败时还能放回去
	stash := execPath + ".rollback-tmp"
	_ = os.Remove(stash)
	if err := os.Rename(execPath, stash); err != nil {
		removeRollbackMarker(execPath)
		return fmt.Errorf("stash current binary: %w", err)
	}
	if err := os.Rename(backup, execPath); err != nil {
		if restoreErr := os.Rename(stash, execPath); restoreErr != nil {
			// 当前二进制与恢复都失败时保留 marker，阻止任何仍在内存里的
			// 新版本继续碰数据库，等待运维人工恢复文件。
			return fmt.Errorf("rollback failed and restore failed: %w (restore: %v)", err, restoreErr)
		}
		removeRollbackMarker(execPath)
		return fmt.Errorf("rollback failed (current binary restored): %w", err)
	}
	_ = os.Remove(stash)
	// 备份已被 rename 消耗掉，元数据描述的对象不复存在，留着只会让下次探测误判还有备份
	removeBackupInfo(execPath)
	return nil
}

// swapBinary 以 rename 两步完成原子替换。
// 不能直接向运行中的可执行文件写入——内核会返回 ETXTBSY；
// 但把它 rename 走是允许的，已 mmap 的旧 inode 继续供当前进程使用。
func swapBinary(execPath, newBinary string) error {
	backup := backupPath(execPath)
	_ = os.Remove(backup)
	// 旧备份及其描述一起失效；若本次元数据写入失败，缺失会被按 unsafe
	// 处理，不能让上一轮 metadata 假装描述新生成的 backup。
	removeBackupInfo(execPath)
	removeRollbackMarker(execPath)

	if err := os.Rename(execPath, backup); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(newBinary, execPath); err != nil {
		if restoreErr := os.Rename(backup, execPath); restoreErr != nil {
			return fmt.Errorf("swap failed and restore failed: %w (restore: %v)", err, restoreErr)
		}
		return fmt.Errorf("swap failed (old binary restored): %w", err)
	}
	return nil
}

// archivePrefix 归档文件名前缀。发行版里同时存在 dujiao-all_* 等其它归档，
// 只按平台后缀匹配会选错文件，必须连项目名一起限定。
const archivePrefix = binaryName + "_"

// selectAssets 从发行版附件中挑出当前平台的归档与校验和文件
func selectAssets(assets []version.Asset) (archiveURL, checksumURL, assetName string, err error) {
	suffix := PlatformAssetSuffix()
	for _, a := range assets {
		switch {
		case isChecksumAsset(a.Name):
			checksumURL = a.DownloadURL
		case strings.HasPrefix(a.Name, archivePrefix) && strings.HasSuffix(a.Name, suffix):
			archiveURL, assetName = a.DownloadURL, a.Name
		}
	}
	if archiveURL == "" {
		return "", "", "", fmt.Errorf("%w: %s%s", ErrAssetNotFound, archivePrefix, suffix)
	}
	return archiveURL, checksumURL, assetName, nil
}

// isChecksumAsset 识别校验和文件。
// goreleaser 默认模板是 {{.ProjectName}}_{{.Version}}_checksums.txt
// （实际发行版里形如 dujiao-next_1.3.1_checksums.txt，版本号不带 v 前缀），
// 同时兼容显式配置成裸 checksums.txt 的情况。
func isChecksumAsset(name string) bool {
	if !strings.HasSuffix(name, "checksums.txt") {
		return false
	}
	return name == "checksums.txt" || strings.HasPrefix(name, archivePrefix)
}

// validateDownloadURL 校验下载地址协议与主机白名单
func validateDownloadURL(u *url.URL) error {
	if u.Scheme != "https" {
		return fmt.Errorf("refuse non-https download url: %s", u.Scheme)
	}
	if !allowedDownloadHosts[u.Hostname()] {
		return fmt.Errorf("refuse download from untrusted host: %s", u.Hostname())
	}
	return nil
}

// download 拉取 rawURL 到 dest，限制最大体积并按 Content-Length 回调进度
func (u *Updater) download(ctx context.Context, rawURL, dest string, maxSize int64, onProgress func(percent int)) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if err := validateDownloadURL(parsed); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", updateUserAgent)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}
	if resp.ContentLength > maxSize {
		return fmt.Errorf("archive too large: %d bytes", resp.ContentLength)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := copyWithProgress(f, io.LimitReader(resp.Body, maxSize+1), resp.ContentLength, onProgress)
	if err != nil {
		return err
	}
	if written > maxSize {
		return fmt.Errorf("archive exceeds %d bytes", maxSize)
	}
	return f.Sync()
}

// copyWithProgress 边拷贝边按总长度回调百分比；total <= 0 时不回调进度
func copyWithProgress(dst io.Writer, src io.Reader, total int64, onProgress func(percent int)) (int64, error) {
	buf := make([]byte, 256<<10)
	var written int64
	lastPercent := -1

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return written, writeErr
			}
			written += int64(n)
			if onProgress != nil && total > 0 {
				percent := int(written * 100 / total)
				if percent > 100 {
					percent = 100
				}
				// 只在整数百分比变化时回调，避免高频写状态
				if percent != lastPercent {
					lastPercent = percent
					onProgress(percent)
				}
			}
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

// verifyChecksum 用发行版的 checksums.txt 校验归档 sha256。
// 这是整条链路唯一的完整性保证：GitHub 走 HTTPS 能防中间人，
// 但防不住附件本身被替换，因此校验值必须来自同一 release 的独立文件。
func (u *Updater) verifyChecksum(ctx context.Context, filePath, assetName, checksumURL string) error {
	parsed, err := url.Parse(checksumURL)
	if err != nil {
		return fmt.Errorf("parse checksum url: %w", err)
	}
	if err := validateDownloadURL(parsed); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return fmt.Errorf("create checksum request: %w", err)
	}
	req.Header.Set("User-Agent", updateUserAgent)

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("fetch checksums returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumSize))
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}

	expected := lookupChecksum(string(body), assetName)
	if expected == "" {
		return fmt.Errorf("%s not listed in checksums.txt", assetName)
	}

	actual, err := fileSHA256(filePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expected, actual)
	}
	return nil
}

// lookupChecksum 解析 sha256sum 格式（"<hex>  <filename>"）取出目标文件的校验值
func lookupChecksum(content, assetName string) string {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		// 部分工具会给二进制模式加 "*" 前缀
		if strings.TrimPrefix(fields[1], "*") == assetName {
			return fields[0]
		}
	}
	return ""
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractBinary 从 tar.gz 归档中取出可执行文件写到 dest。
// 归档里还有 config.yml.example / README.md，只提取 binaryName 一项。
//
// 写入前后都要卡尺寸：归档本身的 sha256 校验发生在解压之前，一个合法且校验通过的
// 归档完全可能解出一个超大条目。如果这里只用 LimitReader 截断，io.Copy 会「成功」返回，
// 一个被砍掉尾巴的二进制就这样通过 chmod 和 swap 上线，直到下次重启才暴露。
func extractBinary(archivePath, dest string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s not found in archive", binaryName)
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		// 只按文件名匹配，忽略归档内层级；同时天然规避 ../ 路径穿越，
		// 因为目标路径是我们自己拼的，从不采用归档里的 header.Name。
		if filepath.Base(header.Name) != binaryName {
			continue
		}

		if header.Size <= 0 || header.Size > maxBinarySize {
			return fmt.Errorf("%s has invalid size %d bytes (limit %d)", binaryName, header.Size, int64(maxBinarySize))
		}

		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return fmt.Errorf("create binary: %w", err)
		}
		defer out.Close()

		// 用 CopyN 按 header 声明的字节数精确读取：短读会返回 io.EOF，
		// 从而把「归档声称有 N 字节但实际只给了 M 字节」这种情况暴露成错误而不是静默截断。
		written, err := io.CopyN(out, tr, header.Size)
		if err != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("write binary (%d of %d bytes): %w", written, header.Size, err)
		}
		if written != header.Size {
			_ = os.Remove(dest)
			return fmt.Errorf("short binary extraction: got %d bytes, want %d", written, header.Size)
		}
		if err := out.Sync(); err != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("sync binary: %w", err)
		}
		return nil
	}
}
