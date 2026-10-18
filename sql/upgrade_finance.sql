-- ========================================
-- 财务报表（收入 / 成本 / 利润）
-- 已存在的库增量执行本文件。
-- ========================================

USE order_ai;

-- 财务报表按 PI 签发月份取汇率，因此汇率按月份存档：
--   exchange_rate        默认汇率（未维护月度汇率的月份使用）
--   exchange_rate_yyyy-MM 某个月的汇率，例如 exchange_rate_2026-09
-- 月度汇率可在「系统设置 → 月度汇率」里维护，也可直接写 app_settings。
INSERT IGNORE INTO app_settings (setting_key, setting_value, remark) VALUES
    ('exchange_rate', '7.1', '默认人民币汇率(1 外币 = N CNY)，未维护月度汇率的月份使用');
