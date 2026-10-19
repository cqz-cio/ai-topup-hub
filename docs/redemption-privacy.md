# 兑换入口仅用于付款后交付

售前页面、公开 API、商城与后台静态脚本都不能包含实际兑换网址。后台页面需要登录，不代表后台 JavaScript 是私有资源。

商品内容继续使用“付款后查看订单交付说明”的表述。实际网址保存在商品 `instructions_json` 和订单交付快照中：订单列表不返回说明，详情仅在 `paid_at` 存在时返回；登录用户必须匹配订单归属，游客必须提供订单邮箱和查询密码。不要把网址加回商品正文、公告、关于页、SEO 或前端常量。

历史商城和后台脚本统一由 `ops/tencent/frontend-gateway/nginx-retired-assets.conf` 撤下，返回 `410` 和 `Cache-Control: no-store`。商城、后台域名和后台 IP 入口均加载此清单。保留服务器私有备份，但不能通过公开路径重新提供这些文件。

新加坡原 IP 和原 HTTPS 入口只保留 API、上传文件、健康检查及证书验证；前端页面与资源跳转到上海当前站点，原管理路径跳转到正式后台。支付通知仍经 `/api/` 转发，上海的私有 SSH API 通道不变。配置为 `ops/tencent/www-qzweb-nginx.conf` 和 `frontend-gateway/nginx-origin-backend.conf`。

发布前构建两套前端，并运行：

```powershell
python -B scripts/audit_public_redemption.py frontend/user/dist frontend/admin/dist
python -B -m unittest discover -s scripts/tests -p test_audit_public_redemption.py
```

检查器覆盖文本、HTML/URL/JavaScript 转义以及 gzip；遇到 Brotli 文件需要安装对应解码器，否则检查失败。它不能替代线上检查：仍需检查公开配置、所有公开商品与文章、旧资源地址、旧 IP 入口，以及未登录和未付款的订单访问。

订单和发货数据只在服务器或隔离测试库核验，不用正式付款、采购或把验收订单改为已付款来测试。已经被第三方下载的历史副本无法通过撤下服务器资源收回。
