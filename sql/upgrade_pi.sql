-- ========================================
-- PI 生成扩展：增量升级脚本
-- 目标库：order_ai
-- 内容与 sql/init.sql 末尾的扩展段一致，供已存在的库增量执行。
-- 执行方式：
--   mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_pi.sql
-- ========================================

-- ----------------------------------------
-- 产品(SKU)扩展：PI 用的简称与美金售价
-- ----------------------------------------
ALTER TABLE skus
    ADD COLUMN short_name VARCHAR(100) NOT NULL DEFAULT ''
        COMMENT 'PI 产品简称,如 V1-8cm blue' AFTER product_name_zh,
    ADD COLUMN series VARCHAR(50) NOT NULL DEFAULT ''
        COMMENT '系列/版本 V1/V2' AFTER short_name,
    ADD COLUMN size_spec VARCHAR(50) NOT NULL DEFAULT ''
        COMMENT '尺寸 6cm/7cm/8cm' AFTER series,
    ADD COLUMN pi_color VARCHAR(50) NOT NULL DEFAULT ''
        COMMENT '客户称呼的颜色 blue/green/orange' AFTER size_spec,
    ADD COLUMN unit VARCHAR(30) NOT NULL DEFAULT 'pcs' AFTER pack_quantity,
    ADD COLUMN hs_code VARCHAR(50) NOT NULL DEFAULT ''
        COMMENT '报关编码' AFTER unit,
    ADD COLUMN category VARCHAR(30) NOT NULL DEFAULT 'finished'
        COMMENT 'finished/packaging/material' AFTER hs_code,
    ADD COLUMN price_usd DECIMAL(15,2) NOT NULL DEFAULT 0
        COMMENT 'PI 美金售价(成本价保留在 price)' AFTER price;

-- ----------------------------------------
-- 材料(包材/原材料)，人工维护
-- ----------------------------------------
CREATE TABLE IF NOT EXISTS materials (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code          VARCHAR(100) NOT NULL DEFAULT '',
    name          VARCHAR(255) NOT NULL DEFAULT '',
    name_zh       VARCHAR(255) NOT NULL DEFAULT '',
    short_name    VARCHAR(100) NOT NULL DEFAULT ''
        COMMENT 'PI 简称,如 V1 7cm box',
    category      VARCHAR(30) NOT NULL DEFAULT 'packaging'
        COMMENT 'packaging(包材) / material(原材料)',
    spec          VARCHAR(255) NOT NULL DEFAULT '',
    unit          VARCHAR(30) NOT NULL DEFAULT 'pcs',
    price         DECIMAL(15,2) NOT NULL DEFAULT 0
        COMMENT 'PI 箱费单价',
    currency      VARCHAR(20) NOT NULL DEFAULT 'CNY',
    supplier_name VARCHAR(255) NOT NULL DEFAULT '',
    safety_stock  DECIMAL(15,3) NOT NULL DEFAULT 0,
    status        VARCHAR(20) NOT NULL DEFAULT 'active',
    remark        TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_material_code (code),
    KEY idx_category (category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ----------------------------------------
-- 发货单(分批发货 + 运费/保险)
-- ----------------------------------------
CREATE TABLE IF NOT EXISTS shipments (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    shipment_number VARCHAR(50) NOT NULL DEFAULT '',
    order_id        BIGINT UNSIGNED NOT NULL,
    batch_no        INT NOT NULL DEFAULT 1 COMMENT '第几批',
    ship_date       DATE NULL,
    shipping_method VARCHAR(100) NOT NULL DEFAULT '',
    carrier         VARCHAR(100) NOT NULL DEFAULT '',
    tracking_no     VARCHAR(100) NOT NULL DEFAULT '',
    carton_count    DECIMAL(15,3) NOT NULL DEFAULT 0,
    freight_cost    DECIMAL(15,2) NOT NULL DEFAULT 0 COMMENT '运费',
    insurance_cost  DECIMAL(15,2) NOT NULL DEFAULT 0 COMMENT '保险',
    currency        VARCHAR(20) NOT NULL DEFAULT 'USD',
    status          VARCHAR(20) NOT NULL DEFAULT 'draft'
        COMMENT 'draft/shipped/delivered',
    remark          TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_shipment_number (shipment_number),
    KEY idx_order (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS shipment_items (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    shipment_id   BIGINT UNSIGNED NOT NULL,
    order_item_id BIGINT UNSIGNED DEFAULT NULL,
    item_type     VARCHAR(20) NOT NULL DEFAULT 'sku' COMMENT 'sku/material',
    sku_id        BIGINT UNSIGNED DEFAULT NULL,
    material_id   BIGINT UNSIGNED DEFAULT NULL,
    short_name    VARCHAR(100) NOT NULL DEFAULT '',
    description   VARCHAR(500) NOT NULL DEFAULT '' COMMENT 'PI 描述整串',
    quantity      DECIMAL(15,3) NOT NULL DEFAULT 0,
    unit          VARCHAR(30) NOT NULL DEFAULT '',
    pack_quantity DECIMAL(15,3) NOT NULL DEFAULT 0,
    carton_count  DECIMAL(15,3) NOT NULL DEFAULT 0,
    source        VARCHAR(20) NOT NULL DEFAULT 'stock' COMMENT 'stock/production',
    unit_price    DECIMAL(15,4) NOT NULL DEFAULT 0,
    amount        DECIMAL(15,2) NOT NULL DEFAULT 0,
    is_paid       TINYINT NOT NULL DEFAULT 0,
    remark        VARCHAR(500) NOT NULL DEFAULT '',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_shipment (shipment_id),
    KEY idx_order_item (order_item_id),
    CONSTRAINT fk_si_shipment
        FOREIGN KEY (shipment_id)
        REFERENCES shipments(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ----------------------------------------
-- 库存台账(流水) + 结存视图
-- ----------------------------------------
CREATE TABLE IF NOT EXISTS inventory_transactions (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    item_type   VARCHAR(20) NOT NULL DEFAULT 'sku' COMMENT 'sku/material',
    item_id     BIGINT UNSIGNED NOT NULL,
    direction   TINYINT NOT NULL DEFAULT 1 COMMENT '1=入库,-1=出库',
    biz_type    VARCHAR(30) NOT NULL DEFAULT ''
        COMMENT 'purchase_in/production_in/shipment_out/consumption/adjust',
    quantity    DECIMAL(15,3) NOT NULL DEFAULT 0,
    unit_cost   DECIMAL(15,4) NOT NULL DEFAULT 0,
    currency    VARCHAR(20) NOT NULL DEFAULT 'CNY',
    warehouse   VARCHAR(50) NOT NULL DEFAULT 'default',
    batch_no    VARCHAR(100) NOT NULL DEFAULT '',
    ref_type    VARCHAR(30) NOT NULL DEFAULT ''
        COMMENT 'purchase_order/production_order/shipment/manual',
    ref_id      BIGINT UNSIGNED DEFAULT NULL,
    occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    operator    VARCHAR(100) NOT NULL DEFAULT '',
    remark      VARCHAR(500) NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_item (item_type, item_id),
    KEY idx_ref (ref_type, ref_id),
    KEY idx_occurred (occurred_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE OR REPLACE VIEW v_inventory_stock AS
SELECT item_type, item_id, warehouse,
       SUM(direction * quantity) AS stock_qty
FROM inventory_transactions
GROUP BY item_type, item_id, warehouse;

-- ----------------------------------------
-- 形式发票(PI) 头 + 明细
-- ----------------------------------------
CREATE TABLE IF NOT EXISTS proforma_invoices (
    id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    pi_number      VARCHAR(50) NOT NULL DEFAULT '',
    customer_name  VARCHAR(255) NOT NULL DEFAULT '',
    issued_date    DATE NULL,
    currency       VARCHAR(20) NOT NULL DEFAULT 'USD',
    status         VARCHAR(20) NOT NULL DEFAULT 'draft'
        COMMENT 'draft/issued/closed',
    item_amount    DECIMAL(15,2) NOT NULL DEFAULT 0,
    box_amount     DECIMAL(15,2) NOT NULL DEFAULT 0,
    insurance_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    freight_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    amount         DECIMAL(15,2) NOT NULL DEFAULT 0,
    stock_note     TEXT,
    remark         TEXT,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_pi_number (pi_number),
    KEY idx_issued (issued_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS proforma_invoice_items (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    pi_id        BIGINT UNSIGNED NOT NULL,
    item_type    VARCHAR(20) NOT NULL DEFAULT 'goods'
        COMMENT 'group/goods/box/charge/note',
    group_no     INT NOT NULL DEFAULT 0,
    group_title  VARCHAR(255) NOT NULL DEFAULT '',
    order_id     BIGINT UNSIGNED DEFAULT NULL,
    shipment_id  BIGINT UNSIGNED DEFAULT NULL,
    sku_id       BIGINT UNSIGNED DEFAULT NULL,
    material_id  BIGINT UNSIGNED DEFAULT NULL,
    model        VARCHAR(100) NOT NULL DEFAULT '',
    short_name   VARCHAR(100) NOT NULL DEFAULT '',
    description  VARCHAR(500) NOT NULL DEFAULT '',
    quantity     DECIMAL(15,3) NOT NULL DEFAULT 0,
    unit         VARCHAR(30) NOT NULL DEFAULT 'pcs',
    unit_price   DECIMAL(15,4) NOT NULL DEFAULT 0,
    amount       DECIMAL(15,2) NOT NULL DEFAULT 0,
    is_paid      TINYINT NOT NULL DEFAULT 0,
    charge_type  VARCHAR(20) NOT NULL DEFAULT ''
        COMMENT 'insurance/shipping/other',
    title        VARCHAR(255) NOT NULL DEFAULT '',
    sort_order   INT NOT NULL DEFAULT 0,
    remark       VARCHAR(500) NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_pi (pi_id),
    CONSTRAINT fk_pii_pi
        FOREIGN KEY (pi_id)
        REFERENCES proforma_invoices(id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
