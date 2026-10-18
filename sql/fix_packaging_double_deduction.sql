-- ============================================================
-- 修正：包装材料被重复扣减，导致库存为负数
-- ============================================================
-- 现象
--   纸箱等包材的结存是负数，采购明明入库过。
--
-- 原因
--   同一只纸箱有两条出库路径同时生效：
--     1) 生产领料页面「载入用料需求」把纸箱也带了出来，手工领料出库
--        → biz_type = 'requisition'
--     2) 发货时按实际发货量自动核销包材
--        → biz_type = 'shipment_pack_out'
--   于是被扣了两次。
--
-- 现行口径（代码已调整）
--   包材只由「发货核销」扣减；生产领料不再领包材。
--   因此需要把历史上误领的那批包材领料流水冲销掉。
--
-- 执行前务必：
--   1) 备份流水表
--      mysqldump -h 127.0.0.1 -uroot -p order_ai inventory_transactions > inventory_transactions_backup.sql
--   2) 先跑第 1 步，确认待冲销清单确实都是这一次的误操作
--   3) 再解开第 2 步的注释执行，最后用第 3 步核对
-- ============================================================


-- ------------------------------------------------------------
-- 第 1 步：列出待冲销的包材领料流水（只看，不改）
-- ------------------------------------------------------------
SELECT
    t.id,
    t.occurred_at,
    m.code,
    COALESCE(NULLIF(m.name_zh, ''), m.name, '') AS material_name,
    t.quantity,
    t.ref_type,
    t.ref_id,
    LEFT(t.remark, 40) AS remark
FROM inventory_transactions t
JOIN materials m ON m.id = t.item_id
WHERE t.item_type = 'material'
  AND t.direction = -1
  AND t.biz_type = 'requisition'
  AND m.category = 'packaging'
ORDER BY m.code, t.id;


-- ------------------------------------------------------------
-- 第 2 步：冲销这些误领流水
--   ※ 确认第 1 步结果无误后，去掉下面 DELETE 语句每行开头的 "-- " 再执行
-- ------------------------------------------------------------
-- DELETE t
-- FROM inventory_transactions t
-- JOIN materials m ON m.id = t.item_id
-- WHERE t.item_type = 'material'
--   AND t.direction = -1
--   AND t.biz_type = 'requisition'
--   AND m.category = 'packaging';


-- ------------------------------------------------------------
-- 第 3 步：核对是否还有负库存（理想结果：0 行）
-- ------------------------------------------------------------
SELECT
    v.item_type,
    v.item_id,
    ROUND(v.stock_qty, 3) AS stock_qty,
    COALESCE(m.code, s.sku_code, '') AS code,
    COALESCE(NULLIF(m.name_zh, ''), NULLIF(m.name, ''),
             NULLIF(s.product_name_zh, ''), '') AS name
FROM v_inventory_stock v
LEFT JOIN materials m ON v.item_type = 'material' AND m.id = v.item_id
LEFT JOIN skus s ON v.item_type = 'sku' AND s.id = v.item_id
WHERE v.stock_qty < -0.0001
ORDER BY v.stock_qty ASC;
