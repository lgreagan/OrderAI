-- 生产单支持"从采购订单下推"：记录来源采购单。
-- 采购到货已入库的简单加工/包装/装箱，生产完工不再重复成品入库。
ALTER TABLE production_orders
    ADD COLUMN source_purchase_order_id BIGINT UNSIGNED DEFAULT NULL
        COMMENT '来源采购订单(从采购下推时写入)' AFTER source_order_id;

ALTER TABLE production_orders
    ADD KEY idx_source_purchase (source_purchase_order_id);
