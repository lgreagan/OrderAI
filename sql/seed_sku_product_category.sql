-- ========================================
-- SKU 挂报关品类（2026-10-17 起按「同海关编码 = 一个品类」挂）+ 删除旧占位品类
-- 前置：先执行 sql/seed_product_categories.sql（现存 5 条品类）
-- 匹配规则：
--   西瓜球 / 专利球全部规格（V1/V2、6/7/8cm、2pack、各颜色） -> CAT-DOG-TREAT-BALL（dog treat ball，HS 4017.00.0000）
--   Dog Lick Bone -> dog bone toys；Dog Puzzle Bowl -> dog puzzle bowl
-- 执行方式：
--   mysql -h 127.0.0.1 -uroot -p order_ai < sql/seed_sku_product_category.sql
-- 说明：按 sku_code 批量 UPDATE（可重复执行）；最后删除 CAT-BALL/CAT-BONE/CAT-BOWL 三条旧占位品类，
--       删除带「仍被 SKU 引用则跳过」的保护条件。
-- ========================================

-- ----------------------------------------
-- 一、SKU 挂品类（14 条；product_name -> 品类英文品名）
-- ----------------------------------------
-- 2026-10-17 起，西瓜球/专利球全部规格（6/7/8cm、V2）统一挂 CAT-DOG-TREAT-BALL 一条品类（同 HS 4017.00.0000）
UPDATE skus s
  JOIN product_categories c ON c.code = 'CAT-DOG-TREAT-BALL'
  SET s.product_category_id = c.id
WHERE s.sku_code IN (
  'B08Y93XMWP','B08Y942TCZ','B08Y94B252','B08Y93BV2V','B08Y94HMMY','B08Y93833P',
  'B0BQPR8XCK','B0BQPPRWBN','B0BQQ1L1NW','B09XJ82B8F','B09XJ77JQX','B09XJ2WLJ7'
);
-- X002MG3TF1  Dog Lick Bone  ->  dog bone toys
UPDATE skus s
  JOIN product_categories c ON c.code = 'CAT-DOG-BONE-TOYS'
  SET s.product_category_id = c.id
WHERE s.sku_code = 'X002MG3TF1';
-- X002IX9VDR  Dog Puzzle Bowl  ->  dog puzzle bowl
UPDATE skus s
  JOIN product_categories c ON c.code = 'CAT-DOG-PUZZLE-BOWL'
  SET s.product_category_id = c.id
WHERE s.sku_code = 'X002IX9VDR';

-- ----------------------------------------
-- 二、删除旧占位品类（已无 SKU 引用；若仍有引用则跳过，改用页面处理）
-- ----------------------------------------
DELETE FROM product_categories
WHERE code IN ('CAT-BALL', 'CAT-BONE', 'CAT-BOWL')
  AND id NOT IN (SELECT DISTINCT product_category_id FROM skus);
