-- ========================================
-- 发货单号修复：回填被置空的 shipment_number
-- 背景：保存发货单时前端不提交 shipment_number，更新逻辑曾将其写为空字符串，
--       导致再次新增/保存时撞上 uk_shipment_number 唯一键
--       （Duplicate entry '' for key 'shipments.uk_shipment_number'）。
--       代码已修复；本脚本用于修好历史脏数据。
-- 目标库：order_ai
-- 执行：mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_shipment_number.sql
-- ========================================

UPDATE shipments
SET shipment_number = CONCAT(
        'SH-',
        DATE_FORMAT(COALESCE(created_at, NOW()), '%Y%m%d'),
        '-',
        LPAD(id, 3, '0')
    )
WHERE shipment_number = '' OR shipment_number IS NULL;
