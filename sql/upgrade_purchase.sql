-- ========================================
-- 采购订单持久化：增量升级脚本
-- 目标库：order_ai
-- 执行：mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_purchase.sql
-- ========================================

CREATE TABLE IF NOT EXISTS purchase_orders (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    po_number       VARCHAR(50) NOT NULL DEFAULT '',
    supplier_name   VARCHAR(255) NOT NULL DEFAULT '',
    source_order_id BIGINT UNSIGNED DEFAULT NULL
        COMMENT '关联的客户订单(可空)',
    order_date      DATE NULL,
    expected_date   DATE NULL,
    currency        VARCHAR(20) NOT NULL DEFAULT 'CNY',
    status          VARCHAR(20) NOT NULL DEFAULT 'draft'
        COMMENT 'draft/ordered/partial/received/cancelled',
    goods_amount    DECIMAL(15,2) NOT NULL DEFAULT 0,
    freight_amount  DECIMAL(15,2) NOT NULL DEFAULT 0,
    total_amount    DECIMAL(15,2) NOT NULL DEFAULT 0,
    remark          TEXT,
    created_by      VARCHAR(100) NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_po_number (po_number),
    KEY idx_source_order (source_order_id),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS purchase_order_items (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    purchase_order_id BIGINT UNSIGNED NOT NULL,
    item_type         VARCHAR(20) NOT NULL DEFAULT 'sku'
        COMMENT 'sku/material',
    item_id           BIGINT UNSIGNED DEFAULT NULL,
    item_code         VARCHAR(100) NOT NULL DEFAULT '',
    item_name         VARCHAR(255) NOT NULL DEFAULT '',
    short_name        VARCHAR(100) NOT NULL DEFAULT '',
    spec              VARCHAR(255) NOT NULL DEFAULT '',
    unit              VARCHAR(30)  NOT NULL DEFAULT '',
    required_quantity DECIMAL(15,3) NOT NULL DEFAULT 0
        COMMENT '系统建议(客户需求)',
    quantity          DECIMAL(15,3) NOT NULL DEFAULT 0
        COMMENT '实际采购数量(可 > 建议)',
    unit_price        DECIMAL(15,4) NOT NULL DEFAULT 0,
    amount            DECIMAL(15,2) NOT NULL DEFAULT 0,
    received_quantity DECIMAL(15,3) NOT NULL DEFAULT 0
        COMMENT '已入库数量',
    remark            VARCHAR(500) NOT NULL DEFAULT '',
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_po (purchase_order_id),
    CONSTRAINT fk_poi_po
        FOREIGN KEY (purchase_order_id)
        REFERENCES purchase_orders(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
