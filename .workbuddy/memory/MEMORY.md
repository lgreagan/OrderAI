# OrderAI 项目长期约定（Go + Echo + MySQL + Bootstrap + jQuery）

## 启动 / 重启服务
- 端口：正式 **8080**，预览 8099。
- **不要用 `go run .` 常驻**：`go run` 有父进程链（shell → go.exe → go.exe → exe），
  后台任务一被回收或重复启动一次，正在跑的实例会被连带杀掉（表现为
  `listen tcp :8080: bind: Only one usage of each socket address`，日志尾部 `exit status 1`）。
- 正确做法：先 `go build -o tmpinspect/OrderAI.exe .`，再 `APP_PORT=8080 ./tmpinspect/OrderAI.exe`。
- 重启前先确认并停掉旧监听：`Get-NetTCPConnection -LocalPort 8080 -State Listen | % OwningProcess`，
  否则新实例 bind 失败会把这个错误再抛一遍。
- 模板在启动时解析并缓存，**改 `templates/*.html` 必须重启**；`web/js/*.js` 是静态文件不需重启，但浏览器要 Ctrl+F5。

## 数据库
- `"C:/Program Files/MySQL/MySQL Server 8.0/bin/mysql.exe" --default-character-set=utf8mb4 -h 127.0.0.1 -P 3306 -u root -p'SZ_lg$2021' order_ai`
- 改动前用 `mysqldump` 备份到 `tmpinspect/`。

## 数据口径（易踩坑）
- `order_items.quantity` 是下单口径（如「480 × 2 Pack」= 960），SKU 真实个数用
  `order_item_matches.required_quantity`；组合行套数 = required ÷ `quantity_per_set`。
- 箱数一律 `math.Ceil`；组合箱规 = `sets_per_carton × quantity_per_set`，单品 = `skus.pack_quantity`。
- 报关：**一行 = 一条发货明细**（按 shipment_items 出行，不按品类聚合）；
  申报单价优先级 `skus.declare_price → product_categories.declare_price → 明细成交单价 → skus.price_usd`。
- 箱号跨行重复是数据事实（同箱装多个 SKU），总箱数按箱号并集算，不累加。

## 环境
- C 盘长期只剩约 2G（99%），`go build` 报 ENOSPC 时先 `go clean -cache`。
- 测试脚本放 `tmpinspect/`，jsdom 用 `NODE_PATH=C:/Users/lg/.workbuddy/binaries/node/workspace/node_modules`。
