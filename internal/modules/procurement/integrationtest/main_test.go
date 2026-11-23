package procurement_test

import (
	"os"
	"testing"

	"github.com/dujiao-next/internal/upstream"
)

// 本包的巡检用例会向上游测试服务器发数百次请求，客户端节流会让整个包跑上几分钟。
// 这里统一关闭节流；限速逻辑本身由 internal/upstream 的用例覆盖。
func TestMain(m *testing.M) {
	restore := upstream.SetRequestPacing(0, 0)
	code := m.Run()
	restore()
	os.Exit(code)
}
