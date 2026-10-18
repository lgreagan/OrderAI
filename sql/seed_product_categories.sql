-- ========================================
-- 报关品类种子数据（按「产品英文品名」粒度：一个英文品名 = 一条品类，中文品名允许重复）
-- 来源一（模板提取）：excel/templates/DE-海运.xlsx 明细 18–28 行、UK--海运.xlsx 明细 19–29 行，各 11 行样例；
-- 来源二（补充规格）：模板样例行里没有、但 SKU 主数据存在的规格（6cm green / 7cm green / V2 6cm green），按同版本同尺寸的兄弟规格补齐。
-- 取值规则：
--   1) 同一英文品名合并为一条（忽略大小写与多余空格）：模板提取 13 条 + 补充 3 条 = 16 条；
--   2) 两份模板取值不一致时以 DE 为准（UK 样例存在错填列：把用途填进了「材质」列、把链接填进了「型号」列）；
--   3) 仅 UK 有的品名只取确定字段（中文品名 / 海关编码 / 申报单价 / 链接），材质与用途留空由业务员补；
--   4) 英文品名统一为小写 + 单空格（V2 版本标记保留大写）；产品链接去掉 /ref= 跟踪参数，
--      样例行把链接错填到了非「销售链接」列的（如 DE 重量列/价格列、UK 型号列）按链接语义纠正过来；
--   5) 申报单价为模板样例值 / 兄弟规格值，实际调价请在「报关品类」页面修改。
--   6) 同一海关编码的规格合并为一条品类（2026-10-17 起）：原 CAT-DOG-TREAT-BALL-* 13 条
--      （6/7/8cm、2pack、V2 各色，HS 4017.00.0000）已合并为单条 CAT-DOG-TREAT-BALL，
--      报关时同 HS 只出一行。新增规格不再单独建品类，直接挂这条。
-- 执行方式（顺序：先本品类脚本，再 sql/seed_sku_product_category.sql 把 SKU 挂上来）：
--   mysql -h 127.0.0.1 -uroot -p order_ai < sql/seed_product_categories.sql
-- 说明：全部用 INSERT IGNORE（code 已存在则不覆盖，避免冲掉页面上后续的修改）；
--       旧占位品类 CAT-BALL / CAT-BONE / CAT-BOWL 由 sql/seed_sku_product_category.sql 删除。
-- ========================================

-- ----------------------------------------
-- 一、模板提取的品类（13 条）
-- ----------------------------------------
INSERT IGNORE INTO product_categories
  (code, name_en, name_zh, material, hs_code, purpose, brand, model, product_url, declare_price, status) VALUES
  ('CAT-PET-TOOTHBRUSH-BLUE', 'pet toothbrush blue', '宠物牙刷', '硅胶', '4017.00.0000', '宠物清洁', '无', '无', 'https://www.amazon.de/-/en/PawsOnly-Toothbrush-Finger-Dental-Care/dp/B09F9XN68W', 1.0000, 'active'),
  ('CAT-DOG-TREAT-BALL', 'dog treat ball', '宠物球', '硬质橡胶', '4017.00.0000', '宠物玩耍', '无', '无', 'https://www.amazon.de/PawsOnly-Leckerli-Spender-Hundespielzeug-Hunde-Puzzle-Spielzeug-Zahnreinigung/dp/B08Y93XMWP', 1.2200, 'active'),
  ('CAT-DOG-PUZZLE-BOWL', 'dog puzzle bowl', '宠物碗', '塑料', '3926909090', '宠物清洁', '无', '无', 'https://www.amazon.de/gp/product/B0GQH6W3T3', 4.0000, 'active'),
  ('CAT-DOG-BONE-TOYS', 'dog bone toys', '宠物骨头玩具', '', '3926909090', '', '无', '无', 'https://www.amazon.co.uk/PawsOnly-Interactive-Treat-Freezable-Enrichment/dp/B0H66B955B', 3.7900, 'active');

-- ----------------------------------------
-- 二、补充规格（原 3 条 6cm green / 7cm green / V2 6cm green —— 2026-10-17 已并入 CAT-DOG-TREAT-BALL，不再单独建）
-- ----------------------------------------
