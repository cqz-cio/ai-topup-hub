# 小π悬浮客服本地开发

开发分支：`codex/customer-support-agent`。沿用创建时基线 `4243550352902139cd9c4d7530615b6a5131c9f2`，未合并主目录的其他草稿。

当前阶段已在本机接入 DeepSeek 自然对话，使用独立客服后端和持久化额度账本；公开商品资料来自已有本机商城 API。配置、运行和限制详见 [DeepSeek 本地调试](customer-support-deepseek.md)。独立分支源码同步 GitHub 供换设备开发，没有合并主线或部署；不读私有订单，不执行支付、采购或其他商城写入。

- 主站经典/Vault 模板共用右下角派大星入口，分销店铺、分销控制台和独立功能页不挂载。
- 点击展开/收起小窗，支持快捷问题、文字输入、思考与回应、人工联系方式和键盘 Escape；手机避开底部导航，短屏优先容纳聊天内容。
- 默认右下角；用鼠标按住派大星可拖动，6px 内的轻微移动仍按点击处理，拖动松手不触发展开/收起。手机保留轻点打开，不通过滑动移动入口。位置只保存在当前页面，刷新回到默认位置；窗口缩放后自动限制在可见区域。拖动后聊天窗优先在角色上方、下方或侧面展开，窄屏空间不足时优先显示聊天内容，收起后恢复角色；气泡避开聊天窗。
- 保留批准的原画、连续动画及托腮遮挡修复。墨镜反光用于眨眼拟态。网格深度确保前臂位于身体前，透明像素不写入深度；肩部保持连接。
- 遵循减少动态效果设置，后台标签暂停动画；WebGL 不可用时显示原画，客服交互仍可使用。
- 当前问答通过后端读取公开资料、调用 DeepSeek；问候、明确天气问法和人工入口等由本地规则处理。需要进一步打磨的业务细则澄清或转人工。具体兑换入口只指向付款后的订单交付说明，不在答案中写域名。
- 主站联系方式：微信 `Cqz0924ttxk`、QQ `1669268759`、Telegram `@czq_piestar`。

## 本地运行

先按 `D:\卡网\docs\local-development.md` 启动已有日常 API，保持本地采购关闭。客服分支不复制配置或数据库。

在本工作区运行：

```powershell
pwsh -NoProfile -File scripts/local/support-agent.ps1 start
pwsh -NoProfile -File scripts/local/support-preview.ps1 start
pwsh -NoProfile -File scripts/local/support-preview.ps1 status
pwsh -NoProfile -File scripts/local/support-preview.ps1 stop
```

入口 `http://127.0.0.1:5175/`，只监听回环地址；复用 `8080` 本机 API。经典/Vault 可用 `?template=classic` / `?template=vault` 检查。PID 与 stdout/stderr 位于工作区忽略目录 `work/codex-logs/`。

前端依赖按 `frontend/user/pnpm-lock.yaml` 安装。当前本机使用与日常目录锁文件一致的依赖目录联接；无新增依赖。后续工作区可正常执行 `pnpm install --frozen-lockfile` 独立安装。

## 验证

`frontend/user` 中执行 `pnpm test`、`pnpm build`；浏览器核对两模板、桌面/窄屏、展开收起、快捷/自由输入、人工联系方式、动态效果降级和托腮前臂完整性。截图和检查报告保存于忽略目录，验证结果见根目录 `design-qa.md` 中的本功能记录。

## 历史第一阶段：商品选购、充值与使用规则流程

实现位于 `frontend/user/src/components/support/supportConversation.ts` 和 `supportCatalog.ts`，对应逻辑测试为 `tests/supportConversation.test.ts`。这一阶段为确定性规则流程；源码与测试保留用于对照，当前小窗已切换到上文的 DeepSeek 接口，以下描述仅记录旧版行为。

- 区分 GPT 订阅、Codex 点数、Gemini 和商城钱包。用当前目录的商品名让客户选择；支持序号、完整名称和商品标识。首次在商品详情提问时读取该页面商品上下文。
- 记住当前商品、选购/使用目标、步骤及 iOS 渠道。覆盖还没买、已付款未交付、已拿到卡密/链接、兑换报错、结果未变化；切换商品或重新开始会清理旧选择。
- 回复中提供站内商品详情、我的订单、游客查单和钱包入口。只导航到现有页面，登录及游客查单继续走现有验证，不在小窗收集查单密码，也不调用私有订单接口。
- `GET /api/v1/public/products` 仅读取公开目录，不携带用户令牌。总读取超时 8 秒，最多 5 页，失败不把部分结果当成完整目录；缓存 60 秒，语言切换清空缓存并取消旧请求。
- 对话只保留校验后的商品 slug、名称、分类和交付类型，不复制商品 HTML、私有交付说明、卡密、售价或实时库存。商品名字来自当前本机接口；本机目录和正式目录独立维护。
- 购买资格摘要来自已有商品规则：`productPurchaseGuide.ts` / `purchaseClarity.ts` 的已确认 SKU 规则，以及 iOS 覆盖充值约束。未知 SKU 不套用新开通或续费的明确资格。改动商品条件时同步维护本模块；最新详情与预检结果仍为准。没有引用旧基线中的硬编码兑换网址。
- 报错仅给排查步骤，不承诺激活成功、自动退款或重发。会明确区分付款、发货和兑换结果；用户输入不上传模型、不落盘、不写日志。

本地可试：`我想买 GPT` → 选择商品 → `1`（购买步骤）→ `已付款但没收到卡密` → `拿到了` → `显示成功但没到账`；或 `买了 iOS 怎么充值` → `3`，检查是否仍保留 iOS 专属流程。


换设备开发与 GitHub Secret 的位置见 [开发密钥清单](development-secrets.md)。Actions Secret 不能下载回本机；初始化脚本使用安全输入生成被 Git 忽略的私密配置。
