-- 生产单增加「作业类型」：把"该不该动库存"从隐式推断改为显式字段。
--
-- produce 投料生产：领原料出库，完工时成品入库
-- pack    包装装箱：采购到货已入库的货，只耗包装材料，完工不再入成品库
-- none    只记录：仅用于排期与统计，完全不涉及库存
--
-- 目标库：order_ai
-- 执行：mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_production_work_type.sql
-- 可重复执行：列/索引已存在时报错可忽略。

ALTER TABLE production_orders
    ADD COLUMN work_type VARCHAR(20) NOT NULL DEFAULT 'produce'
        COMMENT 'produce(投料生产)/pack(包装装箱)/none(只记录)'
        AFTER status;

ALTER TABLE production_orders
    ADD KEY idx_work_type (work_type);

-- 存量数据回填：由采购订单下推的生产单，原本就是"包装/装箱后直接发货"，
-- 其完工不入成品库，统一归为 pack。
UPDATE production_orders
   SET work_type = 'pack'
 WHERE source_purchase_order_id IS NOT NULL
   AND work_type = 'produce';
