package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dujiao-next/internal/admincmd"
	"github.com/dujiao-next/internal/app"
	databasemigrations "github.com/dujiao-next/internal/bootstrap/database/migrations"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/logger"
	adminapplication "github.com/dujiao-next/internal/modules/identity/admin/application"
	adminstore "github.com/dujiao-next/internal/modules/identity/admin/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/selfupdate"
	"github.com/dujiao-next/internal/shared/passwordpolicy"
	"github.com/dujiao-next/internal/version"
	"github.com/dujiao-next/internal/web"

	"github.com/gin-gonic/gin"
)

const (
	ansiReset     = "\033[0m"
	ansiBold      = "\033[1m"
	ansiDim       = "\033[2m"
	ansiGreen     = "\033[32m"
	ansiBlue      = "\033[34m"
	ansiCyan      = "\033[36m"
	ansiBrightMag = "\033[95m"
)

func main() {
	// admin 子命令短路：./dujiao-api admin <subcommand>
	// 在 banner / web 提示 / migrate / default admin / app.Run 之前处理，
	// 避免运维操作时打印一堆无关日志。
	if len(os.Args) >= 2 && os.Args[1] == "admin" {
		runAdminSubcommand(os.Args[2:])
		return
	}

	// 必须排在 config.Load 之前：回滚存在的意义就是新版本起不来，
	// 而起不来的原因很可能正是配置或数据库本身。
	if len(os.Args) >= 2 && os.Args[1] == "rollback" {
		runRollbackCommand(os.Args[2:])
		return
	}

	// 正常服务启动要尽可能早地与 CLI/HTTP 回滚争同一把跨进程锁。
	// 若回滚已经把磁盘二进制换回旧版本，当前这个已加载进内存的新进程必须
	// 在加载配置、连接数据库和执行迁移之前退出，等待 systemd 拉起旧版本。
	startupGuard, err := selfupdate.AcquireStartupGuard(version.Version)
	if err != nil {
		switch {
		case errors.Is(err, selfupdate.ErrStartupSuperseded):
			fmt.Fprintln(os.Stderr, "检测到当前版本已被回滚替换；本进程将在数据库迁移前退出")
			// 正常情况下这个标记会被本次启动消费掉，下一次拉起就是回滚后的版本。
			// 但安装目录不可写时删不掉它，同一句话会无限重复 —— 点名文件路径，
			// 让运维知道该删哪个，而不是对着一句「已被回滚替换」反复重启。
			if execPath, pathErr := selfupdate.ExecutablePath(); pathErr == nil {
				fmt.Fprintf(os.Stderr, "若本条反复出现（多见于安装目录不可写），删除该标记后重启：sudo rm -f %s\n",
					execPath+".rollback.json")
			}
		case errors.Is(err, selfupdate.ErrUpdateInProgress):
			fmt.Fprintf(os.Stderr, "另一个升级或回滚正在进行，本次启动已中止，请稍后重试: %v\n", err)
		default:
			fmt.Fprintf(os.Stderr, "无法可靠记录升级状态，已在数据库迁移前中止启动: %v\n", err)
			printUpdateStateRecoveryHint()
		}
		os.Exit(1)
	}
	defer startupGuard.Release()

	printStartupBanner()

	// 加载配置
	cfg := config.Load()
	logger.Init(cfg.Server.Mode, cfg.Log.ToLoggerOptions())
	stdLog := logger.StdLogger()

	weakSecrets := weakRuntimeSecretNames(cfg)
	if len(weakSecrets) > 0 {
		stdLog.Fatalf("以下运行时密钥过弱、重复或仍为默认值，请配置彼此独立的强随机密钥: %s", strings.Join(weakSecrets, ", "))
	}
	defaultAdminUser, defaultAdminPass := resolveDefaultAdminCredentials(cfg)
	if unsafeBootstrapAdminPassword(cfg, defaultAdminPass) {
		if cfg.Server.Mode == "release" {
			stdLog.Fatalf("bootstrap.default_admin_password 为已知默认值或不符合密码策略；请配置强密码，或留空以跳过默认管理员初始化")
		}
		stdLog.Printf("警告: bootstrap.default_admin_password 为已知默认值或不符合密码策略")
	}

	// admin_path 的校验必须赶在数据库初始化和 AutoMigrate 之前。
	// 它原本发生在路由注册阶段，也就是迁移跑完之后 —— 那样一个配置字符串不合法
	// 就会留下「schema 已经前进、进程却起不来」的半吊子状态，回滚旧二进制还要面对新库。
	// 提前到这里，配置有问题就干净地退出，什么都没动。
	if web.Enabled() {
		if err := web.ValidateAdminPath(cfg.Web.AdminPath); err != nil {
			stdLog.Fatalf("web.admin_path 配置错误: %v", err)
		}
		userLabel := "user (disabled)"
		if !cfg.Web.UserSPADisabled {
			userLabel = "user (/)"
		}
		fmt.Println(ansiGreen + "Embedded SPAs: admin (" + cfg.Web.AdminPath + "), " + userLabel + ansiReset)
	}

	// fullstack 模式下若仍使用默认 admin 路径，提示安全风险
	if web.Enabled() && cfg.Server.Mode == "release" && cfg.Web.AdminPath == "/admin" {
		stdLog.Printf("警告: web.admin_path 仍为默认 /admin，建议修改为不易猜测的路径以降低自动化扫描风险")
	}

	// 自动创建数据目录（fullstack 二进制小白部署免去手动 mkdir）
	for _, dir := range []string{"db", "uploads", "logs"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			stdLog.Printf("警告: 创建目录 %s 失败: %v", dir, err)
		}
	}

	// 初始化数据库
	if err := gormdb.InitDB(cfg.Database.Driver, cfg.Database.DSN, gormdb.DBPoolConfig{
		MaxOpenConns:           cfg.Database.Pool.MaxOpenConns,
		MaxIdleConns:           cfg.Database.Pool.MaxIdleConns,
		ConnMaxLifetimeSeconds: cfg.Database.Pool.ConnMaxLifetimeSeconds,
		ConnMaxIdleTimeSeconds: cfg.Database.Pool.ConnMaxIdleTimeSeconds,
	}, cfg.Server.Mode); err != nil {
		stdLog.Fatalf("数据库初始化失败: %v", err)
	}

	// 在第一条迁移 SQL 之前先持久化 migration_started。若迁移部分成功后报错，
	// 后续 CLI 回滚也必须要求 --force，不能把“没有完整启动”误判成“数据库没动过”。
	// startupGuard 从加载配置前就持有跨进程锁，整个迁移期间 CLI/HTTP 回滚都会被拒绝。
	if err := startupGuard.MarkMigrationStarted(); err != nil {
		stdLog.Printf("记录数据库迁移开始状态失败，已拒绝继续启动: %v", err)
		printUpdateStateRecoveryHint()
		os.Exit(1)
	}

	// 自动迁移数据库表
	if err := databasemigrations.AutoMigrate(); err != nil {
		stdLog.Fatalf("数据库迁移失败: %v", err)
	}

	// 初始化默认管理员账号
	if cfg.Server.Mode == "release" && defaultAdminPass == "" {
		stdLog.Printf("警告: 未设置 DJ_DEFAULT_ADMIN_PASSWORD 且 bootstrap.default_admin_password 为空，已跳过默认管理员初始化")
	} else if err := adminapplication.InitDefaultAdmin(adminstore.New(gormdb.DB), defaultAdminUser, defaultAdminPass); err != nil {
		stdLog.Printf("警告: 初始化默认管理员失败: %v", err)
	}

	// 设置 Gin 模式
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 走到这里说明迁移与启动初始化均已完成，补充“完整启动”诊断状态。
	// migration_started 已在迁移前可靠落盘，因此这里即使写失败也不会错误放行回滚。
	if err := startupGuard.MarkStartupCompleted(); err != nil {
		stdLog.Printf("警告: 记录升级启动完成状态失败（回滚仍保持不安全状态）: %v", err)
	}
	startupGuard.Release()

	// 解析命令行参数
	var mode string
	flag.StringVar(&mode, "mode", app.ModeAll, "启动模式: all (默认), api, worker")
	flag.Parse()

	if err := app.Run(app.Options{
		Config:  cfg,
		Logger:  logger.S(),
		Signals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		Mode:    mode,
	}); err != nil {
		stdLog.Fatalf("服务运行失败: %v", err)
	}

	// 后台一键升级触发的重启也是走 SIGTERM 优雅关闭，走到这里说明已经收尾干净。
	// 但 systemd 把 exit 0 当作 clean exit，官方 unit 的 Restart=on-failure 不会拉起，
	// 服务就此下线。因此这种退出必须换成非零码，让 on-failure / always 都能重新拉起。
	// systemctl stop / Ctrl-C 不会设置这个标记，仍然正常退 0。
	if selfupdate.RestartRequested() {
		stdLog.Printf("自更新重启：以退出码 %d 退出，等待 systemd 拉起新二进制", selfupdate.RestartExitCode)
		os.Exit(selfupdate.RestartExitCode)
	}
}

// printUpdateStateRecoveryHint 在「检测到升级现场、但状态写不下去」时给出可执行的出路。
//
// 这条路径是刻意 fail-closed 的：安装目录里还留着一键升级的备份，程序却无法把
// migration_started 落盘。继续启动会让 AutoMigrate 在无人记录的情况下改动 schema，
// 之后一次普通回滚就会被错误放行。停在这里是对的 —— 但光报一句 permission denied，
// 运维根本不知道该动哪个文件，服务就会一直起不来。
func printUpdateStateRecoveryHint() {
	backup, metadata := "<安装目录>/dujiao-next.backup", "<安装目录>/dujiao-next.backup.json"
	if execPath, err := selfupdate.ExecutablePath(); err == nil {
		backup = execPath + ".backup"
		metadata = execPath + ".backup.json"
	}
	fmt.Fprintf(os.Stderr, `
安装目录里存在一键升级留下的状态文件，但程序无法在该目录写入升级状态。
继续启动会让数据库迁移无人记录，之后的回滚可能被错误放行，因此在动数据库之前停下。

请任选其一：
  1) 把安装目录改成服务账号可写（推荐，保留回滚能力）：
       sudo chown -R <服务账号> %s
  2) 确认不再需要这个回滚点，删掉状态文件后重启：
       sudo rm -f %s %s
`, filepath.Dir(backup), backup, metadata)
}

func printStartupBanner() {
	writeStartupBanner(os.Stdout)
}

func writeStartupBanner(w io.Writer) {
	fmt.Fprintln(w, ansiBrightMag+"╔══════════════════════════════════════════════════════════════════════╗"+ansiReset)
	fmt.Fprintln(w, ansiBrightMag+"║                      🚀 Dujiao-Next 启动中                  	      ║"+ansiReset)
	fmt.Fprintln(w, ansiBrightMag+"╚══════════════════════════════════════════════════════════════════════╝"+ansiReset)
	fmt.Fprintln(w, ansiCyan+"██████╗ ██╗   ██╗     ██╗ █████╗  ██████╗      ███╗   ██╗███████╗██╗  ██╗████████╗"+ansiReset)
	fmt.Fprintln(w, ansiCyan+"██╔══██╗██║   ██║     ██║██╔══██╗██╔═══██╗     ████╗  ██║██╔════╝╚██╗██╔╝╚══██╔══╝"+ansiReset)
	fmt.Fprintln(w, ansiCyan+"██║  ██║██║   ██║     ██║███████║██║   ██║     ██╔██╗ ██║█████╗   ╚███╔╝    ██║   "+ansiReset)
	fmt.Fprintln(w, ansiCyan+"██║  ██║██║   ██║██   ██║██╔══██║██║   ██║     ██║╚██╗██║██╔══╝   ██╔██╗    ██║   "+ansiReset)
	fmt.Fprintln(w, ansiCyan+"██████╔╝╚██████╔╝╚█████╔╝██║  ██║╚██████╔╝     ██║ ╚████║███████╗██╔╝ ██╗   ██║   "+ansiReset)
	fmt.Fprintln(w, ansiCyan+"╚═════╝  ╚═════╝  ╚════╝ ╚═╝  ╚═╝ ╚═════╝      ╚═╝  ╚═══╝╚══════╝╚═╝  ╚═╝   ╚═╝   "+ansiReset)
	fmt.Fprintln(w, ansiGreen+ansiBold+"Open Source Repositories"+ansiReset)
	fmt.Fprintln(w, ansiBlue+"• Organization:  https://github.com/dujiao-next"+ansiReset)
	fmt.Fprintln(w, ansiBlue+"• Main:    		 https://github.com/dujiao-next/dujiao-next"+ansiReset)
	fmt.Fprintln(w, ansiBlue+"• Official:		 https://dujiao-next.com"+ansiReset)
	fmt.Fprintln(w, ansiBlue+"• Discussion Group: https://t.me/dujiaonext_official"+ansiReset)
	fmt.Fprintln(w, ansiGreen+"Version: "+version.Version+ansiReset)
	fmt.Fprintln(w, ansiDim+"--------------------------------------------------------------"+ansiReset)
}

func isWeakSecret(secret string) bool {
	if len(secret) < 32 {
		return true
	}
	normalized := strings.ToLower(secret)
	if strings.Contains(normalized, "change-me") ||
		strings.Contains(normalized, "change-in-production") ||
		strings.Contains(normalized, "your-secret-key") {
		return true
	}
	return false
}

func weakRuntimeSecretNames(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	candidates := []struct {
		name   string
		secret string
	}{
		{name: "app.secret_key", secret: cfg.App.SecretKey},
		{name: "jwt.secret", secret: cfg.JWT.SecretKey},
		{name: "user_jwt.secret", secret: cfg.UserJWT.SecretKey},
	}

	weak := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if isWeakSecret(candidate.secret) {
			weak = append(weak, candidate.name)
		}
	}
	for idx := range candidates {
		secret := strings.TrimSpace(candidates[idx].secret)
		if secret == "" {
			continue
		}
		for other := 0; other < idx; other++ {
			if secret != strings.TrimSpace(candidates[other].secret) {
				continue
			}
			if !containsString(weak, candidates[other].name) {
				weak = append(weak, candidates[other].name)
			}
			if !containsString(weak, candidates[idx].name) {
				weak = append(weak, candidates[idx].name)
			}
		}
	}
	return weak
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func unsafeBootstrapAdminPassword(cfg *config.Config, password string) bool {
	password = strings.TrimSpace(password)
	if password == "" {
		return false
	}
	switch strings.ToLower(password) {
	case "admin", "admin123", "password", "password123", "change-me", "changeme":
		return true
	}
	return cfg == nil || passwordpolicy.Validate(cfg.Security.PasswordPolicy.ValidationPolicy(), password) != nil
}

// runRollbackCommand 处理 ./dujiao-next rollback [--force]，把二进制换回升级前的备份。
//
// 这是「新版本起不来」时唯一还能用的恢复路径：后台的回滚接口挂在 HTTP 服务上，
// 而 HTTP 服务恰恰就是起不来的那部分。所以这里刻意不加载 config、不连数据库，
// 全程只做本地文件操作——配置写错、数据库连不上都不影响恢复。
func runRollbackCommand(args []string) {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	force := fs.Bool("force", false, "数据库迁移已开始或元数据不可信时仍强制回滚（存在不兼容风险）")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	execPath, err := selfupdate.ExecutablePath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "定位可执行文件失败: %v\n", err)
		os.Exit(1)
	}

	info, known, _ := selfupdate.ReadBackupInfo(execPath)
	fmt.Printf("可执行文件: %s\n", execPath)
	switch {
	case !known:
		fmt.Println("备份元数据缺失或损坏，无法判断备份版本，也无法确认数据库结构是否已升级。")
		fmt.Println("这通常出现在从旧版本升级上来的第一次——那次替换由旧程序执行，不会留下元数据。")
	case info.PreviousVersion == "":
		fmt.Printf("当前版本:   %s\n", info.TargetVersion)
		fmt.Println("回滚到:     未知版本（元数据由本版本回填，不含来源版本）")
	default:
		fmt.Printf("当前版本:   %s\n", info.TargetVersion)
		fmt.Printf("回滚到:     %s（%s 完成替换）\n", info.PreviousVersion, info.SwappedAt.Local().Format(time.RFC3339))
	}
	if known && info.NewVersionStarted {
		fmt.Println("注意:       新版本已成功启动过，数据库结构可能已升级，回滚存在不兼容风险")
	} else if known && info.MigrationStarted {
		fmt.Println("注意:       新版本已开始数据库迁移，即使启动失败也可能留下部分新结构")
	}

	switch err := selfupdate.NewUpdater().Rollback(*force); {
	case err == nil:
		fmt.Println("回滚完成。请重启服务使其生效：systemctl restart <你的 unit 名>")
	case errors.Is(err, selfupdate.ErrNoBackup):
		fmt.Fprintf(os.Stderr, "没有可回滚的备份：%s 不存在\n", execPath+".backup")
		os.Exit(1)
	case errors.Is(err, selfupdate.ErrRollbackUnsafe):
		if known {
			fmt.Fprintln(os.Stderr, "已拒绝回滚：新版本已开始迁移或成功启动，数据库结构可能已升级。")
		} else {
			fmt.Fprintln(os.Stderr, "已拒绝回滚：元数据不可信，无法确认数据库结构是否已升级。")
		}
		fmt.Fprintln(os.Stderr, "确认要承担不兼容风险时，请先备份数据库，再执行：dujiao-next rollback --force")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "回滚失败: %v\n", err)
		os.Exit(1)
	}
}

// runAdminSubcommand 处理 ./dujiao-api admin <subcommand>，仅初始化 DB
// 后委托给 internal/admincmd 包，不启动 HTTP / worker / web 等服务。
func runAdminSubcommand(args []string) {
	cfg := config.Load()
	if err := gormdb.InitDB(cfg.Database.Driver, cfg.Database.DSN, gormdb.DBPoolConfig{
		MaxOpenConns:           cfg.Database.Pool.MaxOpenConns,
		MaxIdleConns:           cfg.Database.Pool.MaxIdleConns,
		ConnMaxLifetimeSeconds: cfg.Database.Pool.ConnMaxLifetimeSeconds,
		ConnMaxIdleTimeSeconds: cfg.Database.Pool.ConnMaxIdleTimeSeconds,
	}, cfg.Server.Mode); err != nil {
		fmt.Fprintf(os.Stderr, "init db: %v\n", err)
		os.Exit(1)
	}
	admincmd.Run(args)
}

// resolveDefaultAdminCredentials 解析默认管理员初始化凭据（环境变量优先，其次 config.yml）
func resolveDefaultAdminCredentials(cfg *config.Config) (string, string) {
	user := strings.TrimSpace(os.Getenv("DJ_DEFAULT_ADMIN_USERNAME"))
	pass := strings.TrimSpace(os.Getenv("DJ_DEFAULT_ADMIN_PASSWORD"))
	if cfg == nil {
		return user, pass
	}
	if user == "" {
		user = strings.TrimSpace(cfg.Bootstrap.DefaultAdminUsername)
	}
	if pass == "" {
		pass = strings.TrimSpace(cfg.Bootstrap.DefaultAdminPassword)
	}
	return user, pass
}
