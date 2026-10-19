-- ========================================
-- 数据初始化：清空业务流水数据（保留主数据）
-- 目标库：order_ai
-- 执行：mysql -h 127.0.0.1 -uroot -p order_ai < sql/init_data.sql
-- ========================================
--
-- 用途
--   上线前把试运行期间产生的业务单据清空，只留下主数据与字典数据，
--   让系统以「干净的空库」状态进入正式运营。
--
-- 安全闸门（重要，请勿删除）
--   每条 DELETE 都带 `created_at < '2026-10-30 00:00:00'`：
--   11 月上线后即使误执行本脚本，正式单据（创建时间不早于 2026-10-30）
--   也一条都不会被删。此外脚本开头还有一道整体拦截，到点后运行会直接报错退出。
--   若确需清空正式库，请先确认备份，再自行调整 @CUTOFF。
--
-- 保留（主数据 / 字典 / 模板 / 账号，共 11 张表，本脚本不涉及）
--   skus                   产品 SKU 主数据
--   product_categories     报关品类（申报要素主数据）
--   materials              材料主数据（包材 / 原材料）
--   product_bom            产品用料清单（BOM）
--   product_match_rules    组合商品匹配规则
--   product_match_rule_items 组合商品匹配规则明细
--   shipping_channels      货代渠道字典
--   warehouse_addresses    仓库地址库（FBA）
--   declaration_templates  报关资料模板
--   app_settings           系统设置
--   users                  登录账号（保留，避免清库后默认口令复活）
--
-- 清空（业务流水 / 登录会话，共 17 张表）
--   orders / order_items / ai_results / order_item_matches
--   shipments / shipment_items / declaration_records
--   inventory_transactions
--   purchase_orders / purchase_order_items
--   production_orders / production_order_items
--   proforma_invoices / proforma_invoice_items
--   payments / operation_logs
--   user_sessions          登录会话（清掉后所有人需重新登录）
--
-- 说明
--   * 相关外键均为 ON DELETE CASCADE，删除顺序按「子表 → 父表」排列，
--     即使个别行超出日期条件也不会触发约束报错。
--   * 视图 v_inventory_stock 由 inventory_transactions 汇总而来，清空后自动为空。
--   * 删除在单个事务内完成，中途报错会整体回滚。
--   * 末尾重置自增，使正式数据从 id = 1 开始编号。
--   * 脚本会在会话内临时关闭 safe update mode，避免 Workbench 报错 1175
--     （safe update mode 要求 DELETE 的 WHERE 带主键等值条件）；结尾自动恢复。

USE order_ai;

-- 清除范围：创建时间早于该时刻的流水。改这里即可调整分界点。
SET @CUTOFF = '2026-10-30 00:00:00';

-- ========================================
-- 关闭 safe update mode（会话级）
-- MySQL Workbench / mysql 命令行加 --safe-updates 时默认开启，
-- 它要求 DELETE 的 WHERE 必须带主键等值条件，会拦下本脚本的日期范围删除
-- （报错 1175）。这里临时关闭，脚本结尾再恢复原值。
-- 注意：若在 Workbench 里逐条执行，务必先单独运行这行 SET。
-- ========================================
SET @OLD_SAFE_UPDATES = @@SQL_SAFE_UPDATES;
SET SQL_SAFE_UPDATES = 0;

-- ========================================
-- 安全闸门：2026-10-30 起拒绝执行
-- 日期条件已写在每条 DELETE 上，这里再做一层整体拦截，
-- 避免到点后误执行时「静默跑到一半」。
-- ========================================
SET @guard_sql = IF(
    CURDATE() < '2026-10-30',
    'SELECT ''数据初始化脚本就绪：CUTOFF = 2026-10-30 00:00:00'' AS init_data_status',
    'SELECT 1 FROM `__ABORT__脚本已过期_请不要在2026-10-30之后执行清库__`'
);
PREPARE guard_stmt FROM @guard_sql;
EXECUTE guard_stmt;
DEALLOCATE PREPARE guard_stmt;

START TRANSACTION;

-- ---------- 订单 ----------
DELETE FROM order_item_matches WHERE created_at < @CUTOFF;
DELETE FROM order_items         WHERE created_at < @CUTOFF;
DELETE FROM ai_results          WHERE created_at < @CUTOFF;
DELETE FROM orders              WHERE created_at < @CUTOFF;

-- ---------- 发货 / 报关留档 ----------
DELETE FROM shipment_items      WHERE created_at < @CUTOFF;
DELETE FROM declaration_records WHERE created_at < @CUTOFF;
DELETE FROM shipments           WHERE created_at < @CUTOFF;

-- ---------- 库存台账 ----------
DELETE FROM inventory_transactions WHERE created_at < @CUTOFF;

-- ---------- 采购 ----------
DELETE FROM purchase_order_items WHERE created_at < @CUTOFF;
DELETE FROM purchase_orders      WHERE created_at < @CUTOFF;

-- ---------- 生产 ----------
DELETE FROM production_order_items WHERE created_at < @CUTOFF;
DELETE FROM production_orders      WHERE created_at < @CUTOFF;

-- ---------- 形式发票（PI） ----------
DELETE FROM proforma_invoice_items WHERE created_at < @CUTOFF;
DELETE FROM proforma_invoices      WHERE created_at < @CUTOFF;

-- ---------- 收款 / 操作日志 ----------
DELETE FROM payments       WHERE created_at < @CUTOFF;
DELETE FROM operation_logs WHERE created_at < @CUTOFF;

-- ---------- 登录会话 ----------
-- users 是账号主数据，保留；这里只清会话，正式上线后所有人（含管理员）重新登录。
-- user_sessions 主键 token 为 CHAR(64)、无自增，不需要 AUTO_INCREMENT 归位。
DELETE FROM user_sessions  WHERE created_at < @CUTOFF;

COMMIT;

-- ========================================
-- 自增归位：让正式数据从 id = 1 开始编号
-- 若表内仍有留存行（创建时间 >= CUTOFF），MySQL 会自动跳到 max(id) + 1，不会冲突。
-- ========================================
ALTER TABLE order_item_matches      AUTO_INCREMENT = 1;
ALTER TABLE order_items             AUTO_INCREMENT = 1;
ALTER TABLE ai_results              AUTO_INCREMENT = 1;
ALTER TABLE orders                  AUTO_INCREMENT = 1;

ALTER TABLE shipment_items          AUTO_INCREMENT = 1;
ALTER TABLE declaration_records     AUTO_INCREMENT = 1;
ALTER TABLE shipments               AUTO_INCREMENT = 1;

ALTER TABLE inventory_transactions  AUTO_INCREMENT = 1;

ALTER TABLE purchase_order_items    AUTO_INCREMENT = 1;
ALTER TABLE purchase_orders         AUTO_INCREMENT = 1;

ALTER TABLE production_order_items  AUTO_INCREMENT = 1;
ALTER TABLE production_orders       AUTO_INCREMENT = 1;

ALTER TABLE proforma_invoice_items  AUTO_INCREMENT = 1;
ALTER TABLE proforma_invoices       AUTO_INCREMENT = 1;

ALTER TABLE payments                AUTO_INCREMENT = 1;
ALTER TABLE operation_logs          AUTO_INCREMENT = 1;

-- 恢复 safe update mode 原值
SET SQL_SAFE_UPDATES = @OLD_SAFE_UPDATES;

-- ========================================
-- 执行后自检（可选：取消注释后运行）
-- ========================================
-- SELECT 'orders' AS tbl, COUNT(*) AS rows_left FROM orders
-- UNION ALL SELECT 'order_items',              COUNT(*) FROM order_items
-- UNION ALL SELECT 'ai_results',               COUNT(*) FROM ai_results
-- UNION ALL SELECT 'order_item_matches',       COUNT(*) FROM order_item_matches
-- UNION ALL SELECT 'shipments',                COUNT(*) FROM shipments
-- UNION ALL SELECT 'shipment_items',           COUNT(*) FROM shipment_items
-- UNION ALL SELECT 'declaration_records',      COUNT(*) FROM declaration_records
-- UNION ALL SELECT 'inventory_transactions',   COUNT(*) FROM inventory_transactions
-- UNION ALL SELECT 'purchase_orders',          COUNT(*) FROM purchase_orders
-- UNION ALL SELECT 'purchase_order_items',     COUNT(*) FROM purchase_order_items
-- UNION ALL SELECT 'production_orders',        COUNT(*) FROM production_orders
-- UNION ALL SELECT 'production_order_items',   COUNT(*) FROM production_order_items
-- UNION ALL SELECT 'proforma_invoices',        COUNT(*) FROM proforma_invoices
-- UNION ALL SELECT 'proforma_invoice_items',   COUNT(*) FROM proforma_invoice_items
-- UNION ALL SELECT 'payments',                 COUNT(*) FROM payments
-- UNION ALL SELECT 'operation_logs',           COUNT(*) FROM operation_logs
-- UNION ALL SELECT 'skus',                     COUNT(*) FROM skus
-- UNION ALL SELECT 'product_categories',       COUNT(*) FROM product_categories
-- UNION ALL SELECT 'materials',                COUNT(*) FROM materials
-- UNION ALL SELECT 'product_bom',              COUNT(*) FROM product_bom
-- UNION ALL SELECT 'product_match_rules',      COUNT(*) FROM product_match_rules
-- UNION ALL SELECT 'product_match_rule_items', COUNT(*) FROM product_match_rule_items
-- UNION ALL SELECT 'shipping_channels',        COUNT(*) FROM shipping_channels
-- UNION ALL SELECT 'warehouse_addresses',      COUNT(*) FROM warehouse_addresses
-- UNION ALL SELECT 'declaration_templates',    COUNT(*) FROM declaration_templates
-- UNION ALL SELECT 'app_settings',             COUNT(*) FROM app_settings
-- UNION ALL SELECT 'users',                    COUNT(*) FROM users
-- UNION ALL SELECT 'user_sessions',            COUNT(*) FROM user_sessions;
