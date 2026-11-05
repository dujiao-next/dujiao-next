// Package admincmd 提供 admin 子命令实现，作为运维工具集合：
// 列出管理员、重置 2FA、重置密码。所有命令共享调用方初始化好的
// gormdb.DB，无需重复 config.Load / InitDB。
//
// 入口由 cmd/server/main.go 在检测到 "admin" 子命令时调用 Run(args)，
// 容器只需要 dujiao-api 一个二进制即可执行所有运维操作。
package admincmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/cache"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dujiao-next/internal/constants"
	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	auditloggormstore "github.com/dujiao-next/internal/modules/auditlog/infrastructure/gormstore"
	adminstore "github.com/dujiao-next/internal/modules/identity/admin/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

const minPasswordLength = 8

// Usage 打印 admin 子命令帮助文档。
func Usage() {
	fmt.Fprintln(os.Stderr, `admin 子命令: 后台管理员运维工具

用法:
  dujiao-api admin list-admins                            列出所有管理员
  dujiao-api admin reset-2fa --username <name>            重置指定管理员的 2FA
  dujiao-api admin reset-password --username <name> [--password <new>]
                                                          重置管理员密码（超管忘记密码恢复用）
                                                          不传 --password 时从 stdin 隐藏读入两次确认`)
}

// Run 分发 admin 子命令。args 是去掉 "admin" 之后的参数（os.Args[2:]）。
// caller 需先完成 config.Load + gormdb.InitDB。
//
// 失败时直接 os.Exit(1)，与原 admin-tool 二进制行为保持一致；这是 CLI 运维
// 工具的约定（脚本可靠 exit code 判定），不抛出 error 给调用方。
func Run(args []string) {
	if len(args) < 1 {
		Usage()
		os.Exit(1)
	}
	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "list-admins":
		listAdmins()
	case "reset-2fa":
		username := parseStringFlag(rest, "username")
		if username == "" {
			fmt.Fprintln(os.Stderr, "missing --username")
			Usage()
			os.Exit(1)
		}
		resetTOTP(username)
	case "reset-password":
		username := parseStringFlag(rest, "username")
		password := parseStringFlag(rest, "password")
		if username == "" {
			fmt.Fprintln(os.Stderr, "missing --username")
			Usage()
			os.Exit(1)
		}
		resetPassword(username, password)
	default:
		Usage()
		os.Exit(1)
	}
}

// parseStringFlag 解析 --name value 或 --name=value，未出现返回 ""
func parseStringFlag(args []string, name string) string {
	prefix := "--" + name
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == prefix && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, prefix+"=") {
			return a[len(prefix)+1:]
		}
	}
	return ""
}

func listAdmins() {
	repo := adminstore.New(gormdb.DB)
	admins, err := repo.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "list: %v\n", err)
		os.Exit(1)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tUSERNAME\tIS_SUPER\t2FA_ENABLED\tLAST_LOGIN")
	for _, a := range admins {
		enabled := "no"
		if a.TOTPEnabledAt != nil {
			enabled = "yes (" + a.TOTPEnabledAt.Format("2006-01-02") + ")"
		}
		last := "-"
		if a.LastLoginAt != nil {
			last = a.LastLoginAt.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(w, "%d\t%s\t%t\t%s\t%s\n", a.ID, a.Username, a.IsSuper, enabled, last)
	}
	_ = w.Flush()
}

func resetTOTP(username string) {
	repo := adminstore.New(gormdb.DB)
	logService := auditlogapp.NewAdminLoginService(auditloggormstore.NewAdminLoginStore(gormdb.DB))

	admin, err := repo.GetByUsername(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lookup: %v\n", err)
		os.Exit(1)
	}
	if admin == nil {
		fmt.Fprintf(os.Stderr, "no admin with username=%q\n", username)
		os.Exit(1)
	}
	if err := repo.ClearTOTP(admin.ID); err != nil {
		fmt.Fprintf(os.Stderr, "clear: %v\n", err)
		os.Exit(1)
	}
	dropAdminAuthCache(admin.ID)
	rid := "cli-" + uuid.NewString()
	_ = logService.Record(auditlogapp.AdminLoginRecord{
		AdminID:   admin.ID,
		Username:  admin.Username,
		EventType: constants.AdminLoginEvent2FAResetByAdmin,
		Status:    constants.AdminLoginStatusSuccess,
		ClientIP:  "cli",
		UserAgent: "admin-tool",
		RequestID: rid,
	})
	fmt.Printf("OK: 2FA reset for admin id=%d username=%s at %s\n", admin.ID, admin.Username, time.Now().Format(time.RFC3339))
}

func resetPassword(username, providedPassword string) {
	repo := adminstore.New(gormdb.DB)
	logService := auditlogapp.NewAdminLoginService(auditloggormstore.NewAdminLoginStore(gormdb.DB))

	admin, err := repo.GetByUsername(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lookup: %v\n", err)
		os.Exit(1)
	}
	if admin == nil {
		fmt.Fprintf(os.Stderr, "no admin with username=%q\n", username)
		os.Exit(1)
	}

	newPassword, err := obtainNewPassword(providedPassword)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash: %v\n", err)
		os.Exit(1)
	}

	if err := repo.UpdatePassword(admin.ID, string(hash)); err != nil {
		fmt.Fprintf(os.Stderr, "update: %v\n", err)
		os.Exit(1)
	}
	dropAdminAuthCache(admin.ID)

	rid := "cli-" + uuid.NewString()
	_ = logService.Record(auditlogapp.AdminLoginRecord{
		AdminID:   admin.ID,
		Username:  admin.Username,
		EventType: constants.AdminLoginEventPasswordResetByCLI,
		Status:    constants.AdminLoginStatusSuccess,
		ClientIP:  "cli",
		UserAgent: "admin-tool",
		RequestID: rid,
	})
	fmt.Printf("OK: password reset for admin id=%d username=%s at %s\n", admin.ID, admin.Username, time.Now().Format(time.RFC3339))
	fmt.Println("提示: 该管理员所有现有会话已强制下线，请用新密码重新登录。")
}

// obtainNewPassword 决定新密码来源：
//   - 命令行 --password 直接提供时校验后返回
//   - 否则从 stdin 隐藏读取两次确认
func obtainNewPassword(provided string) (string, error) {
	if provided != "" {
		if err := sanityCheckPassword(provided); err != nil {
			return "", err
		}
		return provided, nil
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// 非终端环境（pipe / CI）允许从 stdin 读一行明文，方便脚本化
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		pwd := strings.TrimRight(line, "\r\n")
		if err := sanityCheckPassword(pwd); err != nil {
			return "", err
		}
		return pwd, nil
	}

	fmt.Print("新密码: ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	fmt.Print("再次输入: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password confirmation: %w", err)
	}
	if string(first) != string(second) {
		return "", errors.New("两次输入不一致")
	}
	pwd := string(first)
	if err := sanityCheckPassword(pwd); err != nil {
		return "", err
	}
	return pwd, nil
}

// sanityCheckPassword 仅做最低限度校验（长度），CLI 紧急恢复场景不强制
// 后台密码策略；建议运维登录后立即在管理界面改成符合策略的强密码。
func sanityCheckPassword(pwd string) error {
	if pwd == "" {
		return errors.New("密码不能为空")
	}
	if len([]rune(pwd)) < minPasswordLength {
		return fmt.Errorf("密码长度至少 %d 个字符", minPasswordLength)
	}
	return nil
}

// dropAdminAuthCache 清掉管理员鉴权缓存，让被盗令牌立即失效；
// Redis 不可用时提示缓存最多保留 10 分钟。
func dropAdminAuthCache(adminID uint) {
	if !cache.Enabled() {
		return
	}
	if err := cache.DelAdminAuthState(context.Background(), adminID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: clear auth cache failed (%v); existing sessions may stay valid for up to 10 minutes\n", err)
	}
}
