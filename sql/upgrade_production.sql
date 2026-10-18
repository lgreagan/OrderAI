-- ========================================
-- 生产订单持久化 + 产品用料清单(BOM)：增量升级脚本
-- 目标库：order_ai
-- 执行：mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_production.sql
-- ========================================

CREATE TABLE IF NOT EXISTS production_orders (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    po_number       VARCHAR(50) NOT NULL DEFAULT '',
    source_order_id BIGINT UNSIGNED DEFAULT NULL,
    plan_date       DATE NULL,
    start_date      DATE NULL,
    finish_date     DATE NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'draft'
        COMMENT 'draft/producing/completed/cancelled',
    remark          TEXT,
    created_by      VARCHAR(100) NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_prod_number (po_number),
    KEY idx_source_order (source_order_id),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS production_order_items (
    id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    production_order_id BIGINT UNSIGNED NOT NULL,
    product_type        VARCHAR(20) NOT NULL DEFAULT 'rule'
        COMMENT 'rule(组合套) / sku(单品)',
    product_ref_id      BIGINT UNSIGNED DEFAULT NULL
        COMMENT 'rule_id 或 sku_id',
    product_name        VARCHAR(255) NOT NULL DEFAULT '',
    short_name          VARCHAR(100) NOT NULL DEFAULT '',
    spec                VARCHAR(255) NOT NULL DEFAULT '',
    planned_quantity    DECIMAL(15,3) NOT NULL DEFAULT 0
        COMMENT '计划生产量(套/个)',
    required_quantity   DECIMAL(15,3) NOT NULL DEFAULT 0
        COMMENT '客户需求(系统建议)',
    produced_quantity   DECIMAL(15,3) NOT NULL DEFAULT 0
        COMMENT '已生产量',
    sets_per_carton     DECIMAL(15,3) NOT NULL DEFAULT 0,
    carton_count        DECIMAL(15,3) NOT NULL DEFAULT 0,
    unit                VARCHAR(30)  NOT NULL DEFAULT '',
    remark              VARCHAR(500) NOT NULL DEFAULT '',
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_prod (production_order_id),
    CONSTRAINT fk_poi_prod
        FOREIGN KEY (production_order_id)
        REFERENCES production_orders(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS product_bom (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    product_type      VARCHAR(20) NOT NULL DEFAULT 'rule'
        COMMENT 'rule / sku',
    product_ref_id    BIGINT UNSIGNED NOT NULL,
    material_id       BIGINT UNSIGNED NOT NULL,
    quantity_per_unit DECIMAL(15,6) NOT NULL DEFAULT 1
        COMMENT '每单位产品消耗的材料数量',
    unit              VARCHAR(30) NOT NULL DEFAULT '',
    remark            VARCHAR(500) NOT NULL DEFAULT '',
    status            VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_bom (product_type, product_ref_id, material_id),
    KEY idx_material (material_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 用量精度提升到 6 位小数（1/每箱 更精确，如下推的纸箱用料）。
ALTER TABLE product_bom
    MODIFY quantity_per_unit DECIMAL(15,6) NOT NULL DEFAULT 1
    COMMENT '每单位产品消耗的材料数量';
