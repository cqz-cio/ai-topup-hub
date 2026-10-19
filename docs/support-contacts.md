# 客服联系方式

微信 **Cqz0924ttxk**、QQ **1669268759**、Telegram **@czq_piestar**。

Telegram 链接为 `https://t.me/czq_piestar`。现有微信入口保留；新 QQ 可复制，Telegram 在新标签页打开。

首页与商品中心客服区、经典与 Vault 页脚、关于页、购买资格提示、Codex 说明、已付款订单开票区、付款余额拦截弹窗，以及后台系统信息与运营入口共用联系方式。默认值与安全链接解析位于 `frontend/shared/supportContacts.ts`，展示组件为 `frontend/user/src/components/SupportContacts.vue`。分销商户不继承主站的默认客服信息；站点已有 Telegram 设置继续优先。

公告弹窗、公告文章、两篇 USDT 付款教程、商品公开说明和商品未来交付说明属于数据库内容，须分别更新。使用 `scripts/support_contacts.py` 先预览，再指定备份目录应用。迁移只补充现有微信信息旁的 QQ 和 Telegram，并更新站点 Telegram 设置及关于页联系提示；不改历史订单快照、价格、卡密、支付配置或其他内容。

```powershell
python -B scripts/support_contacts.py --database work/codex-runtime/live/shop.db
python -B scripts/support_contacts.py --database work/codex-runtime/live/shop.db --apply --backup-dir work/codex-runtime/support-contacts/local-backup
```

迁移先做一致性备份，在单个事务内核对所有表和字段，异常时回滚；重复执行不再产生更新。商品公开内容继续排除兑换网址，实际兑换入口仍仅在付款后订单交付说明中提供。

本次验证与发布的私有证据保存在 `work/codex-runtime/support-contacts/`；正式发布号、构建来源和核验结果以 `ops/tencent/production-baseline.json` 为准。
