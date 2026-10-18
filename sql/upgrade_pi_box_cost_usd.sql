-- ========================================
-- PI 箱费单价人民币 → 美元折算（2026-10-18）
-- 背景：purchase_order_items.unit_price / materials.price 是人民币纸箱价，
--       生成 PI 时直接抄进 PI 明细，导致箱费按人民币金额出现在美元发票上。
-- 代码侧：services/pi_prices.go convertBoxCostToUSD 在 Generate 时按
--       「签发月份月度汇率 → 默认汇率」折算；本脚本用于修补历史 PI。
-- 执行方式：
--   mysql -h 127.0.0.1 -P 3306 -uroot -p order_ai < sql/upgrade_pi_box_cost_usd.sql
-- 说明：把 @rate 改成目标 PI 签发月份的汇率；按行重算金额与 PI 抬头合计。
--       脚本可重复执行会有二次折算风险，请先备份、只对未折算的 PI 跑一次。
-- ========================================

SET @rate = 6.69;   -- 2026-10 月度汇率（exchange_rate_2026-10）
SET @pi_id = 1;     -- 目标 PI

START TRANSACTION;

-- 1) 箱费单价按汇率折算成美元（保留 2 位小数，保证 数量 × 单价 = 金额）
UPDATE proforma_invoice_items
   SET unit_price = ROUND(unit_price / @rate, 2)
 WHERE item_type = 'box'
   AND pi_id = @pi_id;

-- 2) 金额按折算后单价重算
UPDATE proforma_invoice_items
   SET amount = ROUND(quantity * unit_price, 2)
 WHERE item_type = 'box'
   AND pi_id = @pi_id;

-- 3) PI 抬头合计重算（箱费只统计未付款行）
UPDATE proforma_invoices p
   SET box_amount = (
         SELECT COALESCE(SUM(amount), 0)
           FROM proforma_invoice_items
          WHERE pi_id = p.id AND item_type = 'box' AND is_paid = 0
       ),
       amount = item_amount
              + (SELECT COALESCE(SUM(amount), 0)
                   FROM proforma_invoice_items
                  WHERE pi_id = p.id AND item_type = 'box' AND is_paid = 0)
              + insurance_amount + freight_amount,
       total_unpaid = item_amount
              + (SELECT COALESCE(SUM(amount), 0)
                   FROM proforma_invoice_items
                  WHERE pi_id = p.id AND item_type = 'box' AND is_paid = 0)
              + insurance_amount + freight_amount + carryover_amount - paid_amount
 WHERE p.id = @pi_id;

COMMIT;

SELECT id, item_amount, box_amount, insurance_amount, freight_amount, amount, total_unpaid
  FROM proforma_invoices
 WHERE id = @pi_id;
