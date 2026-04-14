# demo-feature

这是一个最小演示插件，用于验证以下能力：

- 插件入口导出
- 公共路由返回
- 后台路由返回
- 页面元数据登记
- 事件订阅登记
- 插件独立 SQLite 初始化
- 宿主主库能力探测（当插件声明 `plugin.db.main` 或 `plugin.host.privileged` 时）

构建方式请执行：

```bash
bash api/scripts/build-plugin-demo.sh
```
