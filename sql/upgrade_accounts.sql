-- ========================================
-- 收款 + 系统设置 + PI 余额字段：增量升级脚本
-- 目标库：order_ai
-- 执行：mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_accounts.sql
-- ========================================

ALTER TABLE proforma_invoices
    ADD COLUMN carryover_amount DECIMAL(15,2) NOT NULL DEFAULT 0
        COMMENT '历史欠款(上期结余)' AFTER amount,
    ADD COLUMN total_unpaid DECIMAL(15,2) NOT NULL DEFAULT 0
        COMMENT '累计未收 = 本期合计 + 历史欠款 − 已收款' AFTER carryover_amount,
    ADD COLUMN paid_amount DECIMAL(15,2) NOT NULL DEFAULT 0
        COMMENT '已收款' AFTER total_unpaid;

CREATE TABLE IF NOT EXISTS payments (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    customer_name VARCHAR(255) NOT NULL DEFAULT '',
    pi_id         BIGINT UNSIGNED DEFAULT NULL
        COMMENT '关联的形式发票(可空)',
    amount        DECIMAL(15,2) NOT NULL DEFAULT 0,
    currency      VARCHAR(20) NOT NULL DEFAULT 'USD',
    paid_date     DATE NULL,
    method        VARCHAR(50) NOT NULL DEFAULT '',
    reference     VARCHAR(100) NOT NULL DEFAULT '',
    remark        VARCHAR(500) NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_customer (customer_name),
    KEY idx_pi (pi_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS app_settings (
    setting_key   VARCHAR(100) NOT NULL,
    setting_value TEXT,
    remark        VARCHAR(255) NOT NULL DEFAULT '',
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (setting_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO app_settings (setting_key, setting_value, remark) VALUES
    ('default_currency', 'USD', 'PI/发货默认币种'),
    ('default_warehouse', 'default', '默认仓库'),
    ('pi_terms', '', 'PI 付款条款(打印在 PI 底部)'),
    ('pi_valid_days', '15', 'PI 有效期(天)'),
    ('stock_note_header', '-- 当前库存（自动生成，可修改）--', '库存说明默认抬头'),
    ('low_stock_threshold', '0', '低库存阈值(库存 <= 该值时在统计中列出)');
