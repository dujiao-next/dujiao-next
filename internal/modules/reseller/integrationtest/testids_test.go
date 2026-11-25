package integrationtest

import (
	"sync/atomic"
	"time"
)

// 测试内唯一 ID 生成器。
// 旧代码直接用 time.Now().UnixNano() 拼内存库名、邮箱、订单号，Windows 上时钟精度
// 不足时连续调用会拿到相同的值，导致内存库被多个测试共享、邮箱/订单号唯一约束冲突。
// 这里用「包加载时的基准时间 + 进程内自增」，保证同一次测试运行里不会重复。
var (
	testIDBase = time.Now().UnixNano()
	testIDSeq  atomic.Int64
)

func uniqueTestID() int64 {
	return testIDBase + testIDSeq.Add(1)
}
