-- ========================================
-- SKU 级别「报关申报单价」
--
-- 背景：报关资料改为按发货明细逐行出行后，同一报关品类下的不同规格
--       （如 6cm / 8cm）需要各按各的申报单价申报，品类上的 declare_price
--       只能给一个默认值，因此在 SKU 上增加自己的申报单价。
-- 取值优先级（services/DeclarationService.buildLineFromItem）：
--       skus.declare_price > product_categories.declare_price > 明细成交单价 > skus.price_usd
-- 说明：设为 0 表示「不单独指定」，继续沿用报关品类的默认申报价。
-- 执行：
--   mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_sku_declare_price.sql
-- ========================================

ALTER TABLE skus
  ADD COLUMN declare_price DECIMAL(15,4) NOT NULL DEFAULT 0
  COMMENT '报关申报单价(USD)，0 表示按报关品类的默认申报价' AFTER price_usd;

-- 回填：2026-10-17 品类合并前，各规格在 product_categories 上的申报价。
UPDATE skus SET declare_price = CASE
  WHEN product_name LIKE 'V1 Treat Dispenser Ball%6 cm%' THEN 1.22
  WHEN product_name LIKE 'V1 Treat Dispenser Ball%7 cm%' THEN 1.22
  WHEN product_name LIKE 'V1 Treat Dispenser Ball%8 cm%' THEN 1.96
  WHEN product_name LIKE 'V2 Treat Dispenser Ball%6 cm%' THEN 1.06
  WHEN product_name LIKE 'V2 Treat Dispenser Ball%8 cm%' THEN 2.09
  WHEN sku_code = 'X002MG3TF1' THEN 3.79  -- 骨头 dog bone toys
  WHEN sku_code = 'X002IX9VDR' THEN 4.00  -- 慢食碗 dog puzzle bowl
  WHEN sku_code IN ('B0BBNDPFG7', 'B0BBNCHVNZ') THEN 0.28 -- 刷子 Lint Remover
  WHEN sku_code = 'B092TQGFYY' THEN 1.00  -- 牙刷
  ELSE declare_price
END
WHERE product_category_id > 0;
