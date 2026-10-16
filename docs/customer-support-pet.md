# 小π悬浮客服本地开发

开发分支：`codex/customer-support-agent`。沿用创建时基线 `4243550352902139cd9c4d7530615b6a5131c9f2`，未合并主目录的其他草稿。

当前阶段只实现商城 UI 和本地演示问答，没有模型请求、客服后端、订单查询、支付或其他业务写入。没有部署。会话只保存在组件内存，刷新后清空。

- 主站经典/Vault 模板共用右下角派大星入口，分销店铺、分销控制台和独立功能页不挂载。
- 点击展开/收起小窗，支持快捷问题、文字输入、思考与回应、人工联系方式和键盘 Escape；手机避开底部导航，短屏优先容纳聊天内容。
- 默认右下角；用鼠标按住派大星可拖动，6px 内的轻微移动仍按点击处理，拖动松手不触发展开/收起。手机保留轻点打开，不通过滑动移动入口。位置只保存在当前页面，刷新回到默认位置；窗口缩放后自动限制在可见区域。拖动后聊天窗优先在角色上方、下方或侧面展开，窄屏空间不足时优先显示聊天内容，收起后恢复角色；气泡避开聊天窗。
- 保留批准的原画、连续动画及托腮遮挡修复。墨镜反光用于眨眼拟态。网格深度确保前臂位于身体前，透明像素不写入深度；肩部保持连接。
- 遵循减少动态效果设置，后台标签暂停动画；WebGL 不可用时显示原画，客服交互仍可使用。
- 回复仅按本地预设关键词选择，不推断真实订单、金额、库存、资格或返利；未知问题说明演示限制。具体兑换入口只指向付款后的订单交付说明，不在演示答案中写域名。
- 主站联系方式：微信 `Cqz0924ttxk`、QQ `1669268759`、Telegram `@czq_piestar`。

## 本地运行

先按 `D:\卡网\docs\local-development.md` 启动已有日常 API，保持本地采购关闭。客服分支不复制配置或数据库。

在本工作区运行：

```powershell
pwsh -NoProfile -File scripts/local/support-preview.ps1 start
pwsh -NoProfile -File scripts/local/support-preview.ps1 status
pwsh -NoProfile -File scripts/local/support-preview.ps1 stop
```

入口 `http://127.0.0.1:5175/`，只监听回环地址；复用 `8080` 本机 API。经典/Vault 可用 `?template=classic` / `?template=vault` 检查。PID 与 stdout/stderr 位于工作区忽略目录 `work/codex-logs/`。

前端依赖按 `frontend/user/pnpm-lock.yaml` 安装。当前本机使用与日常目录锁文件一致的依赖目录联接；无新增依赖。后续工作区可正常执行 `pnpm install --frozen-lockfile` 独立安装。

## 验证

`frontend/user` 中执行 `pnpm test`、`pnpm build`；浏览器核对两模板、桌面/窄屏、展开收起、快捷/自由输入、人工联系方式、动态效果降级和托腮前臂完整性。截图和检查报告保存于忽略目录，验证结果见根目录 `design-qa.md` 中的本功能记录。
