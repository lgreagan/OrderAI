-- ========================================
-- SKU 排序：增量升级脚本
-- 目标库：order_ai
-- 内容：skus 增加 sort_order（产品列表统一按它升序展示，值越小越靠前），
--       并把已有产品按业务顺序初始化一次。
-- 执行方式：
--   mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_sku_sort_order.sql
-- 说明：ALTER 段与 sql/init.sql 末尾一致（新库由 init.sql 建列）；
--       下面的 UPDATE 段是给已有数据的库做一次性排序，可重复执行。
--       重复执行 ALTER 段会因列已存在而报错，只需执行 UPDATE 段即可重新排序。
-- ========================================

-- ----------------------------------------
-- 产品(SKU)排序值：产品列表统一按它升序展示
-- 内容与 sql/init.sql 末尾的排序段一致。
-- ----------------------------------------
ALTER TABLE skus
    ADD COLUMN sort_order INT NOT NULL DEFAULT 0
        COMMENT '产品排序值，产品列表按升序展示(越小越靠前)' AFTER id,
    ADD KEY idx_sku_sort_order (sort_order);

-- ----------------------------------------
-- 初始化已有产品的排序值（按 sku_code 定位，重复执行只会重写同样的值）
-- 业务顺序：
--   Dog Lick Bone
--   → V1 blue 6/7/8cm → V1 green 6/7/8cm → V1 orange 6/7/8cm
--   → V2 blue 6/7/8cm → V2 green 6/7/8cm → V2 orange 6/7/8cm
--   → Dog Puzzle Bowl
-- 每两个相邻产品间隔 10，中间留出的号段预留给暂未建档的规格
-- （V1 orange 80/90/100，V2 各色 7cm 120/150/180），以后建好档把值填进去即可。
-- ----------------------------------------
UPDATE skus SET sort_order = 10  WHERE sku_code = 'X002MG3TF1'; -- Dog Lick Bone
UPDATE skus SET sort_order = 20  WHERE sku_code = 'B08Y93XMWP'; -- V1 blue  6cm
UPDATE skus SET sort_order = 30  WHERE sku_code = 'B08Y94B252'; -- V1 blue  7cm
UPDATE skus SET sort_order = 40  WHERE sku_code = 'B08Y94HMMY'; -- V1 blue  8cm
UPDATE skus SET sort_order = 50  WHERE sku_code = 'B08Y942TCZ'; -- V1 green 6cm
UPDATE skus SET sort_order = 60  WHERE sku_code = 'B08Y93BV2V'; -- V1 green 7cm
UPDATE skus SET sort_order = 70  WHERE sku_code = 'B08Y93833P'; -- V1 green 8cm
-- V1 orange 暂未建档：预留 80 / 90 / 100
UPDATE skus SET sort_order = 110 WHERE sku_code = 'B0BQPR8XCK'; -- V2 blue  6cm
-- V2 blue 7cm 暂未建档：预留 120
UPDATE skus SET sort_order = 130 WHERE sku_code = 'B09XJ82B8F'; -- V2 blue  8cm
UPDATE skus SET sort_order = 140 WHERE sku_code = 'B0BQPPRWBN'; -- V2 green 6cm
-- V2 green 7cm 暂未建档：预留 150
UPDATE skus SET sort_order = 160 WHERE sku_code = 'B09XJ2WLJ7'; -- V2 green 8cm
UPDATE skus SET sort_order = 170 WHERE sku_code = 'B0BQQ1L1NW'; -- V2 orange 6cm
-- V2 orange 7cm 暂未建档：预留 180
UPDATE skus SET sort_order = 190 WHERE sku_code = 'B09XJ77JQX'; -- V2 orange 8cm
UPDATE skus SET sort_order = 200 WHERE sku_code = 'X002IX9VDR'; -- Dog Puzzle Bowl
