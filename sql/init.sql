CREATE DATABASE IF NOT EXISTS order_ai
DEFAULT CHARACTER SET utf8mb4
DEFAULT COLLATE utf8mb4_unicode_ci;

USE order_ai;


-- ========================================
-- 订单主表
-- ========================================

CREATE TABLE IF NOT EXISTS orders (

    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    order_number VARCHAR(100) NOT NULL DEFAULT '',

    country VARCHAR(100) NOT NULL DEFAULT '',

    shipping_method VARCHAR(100) NOT NULL DEFAULT '',

    customer VARCHAR(255) NOT NULL DEFAULT '',

    status VARCHAR(30) NOT NULL DEFAULT 'confirmed',

    source_image VARCHAR(500) NOT NULL DEFAULT '',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    KEY idx_order_number (order_number),

    KEY idx_status (status),

    KEY idx_created_at (created_at)

) ENGINE=InnoDB
DEFAULT CHARSET=utf8mb4;


-- ========================================
-- 订单明细
-- ========================================

CREATE TABLE IF NOT EXISTS order_items (

    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    order_id BIGINT UNSIGNED NOT NULL,

    line_number INT NOT NULL DEFAULT 0,

    product_name VARCHAR(500) NOT NULL DEFAULT '',

    description TEXT,

    specification VARCHAR(500) NOT NULL DEFAULT '',

    quantity DECIMAL(15,3) NOT NULL DEFAULT 0,

    unit VARCHAR(50) NOT NULL DEFAULT '',

    pack_size DECIMAL(15,3) NOT NULL DEFAULT 1,

    actual_quantity DECIMAL(15,3) NOT NULL DEFAULT 0,

    declared_quantity VARCHAR(500) NOT NULL DEFAULT '',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    KEY idx_order_id (order_id),

    CONSTRAINT fk_order_items_order

        FOREIGN KEY (order_id)

        REFERENCES orders(id)

        ON DELETE CASCADE

) ENGINE=InnoDB
DEFAULT CHARSET=utf8mb4;


-- ========================================
-- AI识别记录
-- ========================================

CREATE TABLE IF NOT EXISTS ai_results (

    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    order_id BIGINT UNSIGNED NULL,

    provider VARCHAR(50) NOT NULL DEFAULT 'deepseek',

    model VARCHAR(100) NOT NULL DEFAULT '',

    image_path VARCHAR(500) NOT NULL DEFAULT '',

    raw_json LONGTEXT,

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    KEY idx_order_id (order_id)

) ENGINE=InnoDB
DEFAULT CHARSET=utf8mb4;

-- ----------------------------------------
-- SKU表
CREATE TABLE `skus` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `sku_code` varchar(100) NOT NULL DEFAULT '',
  `product_name` varchar(255) NOT NULL DEFAULT '',
  `product_name_zh` varchar(255) NOT NULL DEFAULT '' COMMENT '产品中文名',
  `model` varchar(255) NOT NULL DEFAULT '',
  `price` decimal(15,2) NOT NULL DEFAULT '0.00',
  `currency` varchar(20) NOT NULL DEFAULT 'CNY',
  `image` varchar(500) NOT NULL DEFAULT '',
  `color` varchar(100) NOT NULL DEFAULT '',
  `pack_quantity` decimal(15,3) NOT NULL DEFAULT '1.000',
  `box_length` decimal(10,2) NOT NULL DEFAULT '0.00',
  `box_width` decimal(10,2) NOT NULL DEFAULT '0.00',
  `box_height` decimal(10,2) NOT NULL DEFAULT '0.00',
  `box_unit` varchar(20) NOT NULL DEFAULT 'cm',
  `box_weight` decimal(10,3) NOT NULL DEFAULT '0.000',
  `weight_unit` varchar(20) NOT NULL DEFAULT 'kg',
  `status` varchar(30) NOT NULL DEFAULT 'active',
  `description` text,
  `remark` text,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sku_code` (`sku_code`),
  KEY `idx_product_name` (`product_name`),
  KEY `idx_model` (`model`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB AUTO_INCREMENT=14 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;


-- -----------------------------------------
-- 组合商品匹配规则表

CREATE TABLE IF NOT EXISTS product_match_rules (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    rule_code VARCHAR(100) NOT NULL DEFAULT '',
    customer_product_name VARCHAR(255) NOT NULL DEFAULT ''
        COMMENT '客户订单商品名称',
    customer_specification VARCHAR(255) NOT NULL DEFAULT ''
        COMMENT '客户订单规格，按原始识别结果匹配',
    product_name_zh VARCHAR(255) NOT NULL DEFAULT ''
        COMMENT '产品中文名',
    order_unit VARCHAR(30) NOT NULL DEFAULT '套'
        COMMENT '客户订单计量单位',
    sets_per_carton DECIMAL(15,3) NOT NULL DEFAULT 40
        COMMENT '组合商品每箱套数，本例为40套',
    carton_weight DECIMAL(10,3) NOT NULL DEFAULT 0
        COMMENT '每箱重量（kg）',
    box_length DECIMAL(10,2) NOT NULL DEFAULT 0
        COMMENT '箱子长度cm',
    box_width DECIMAL(10,2) NOT NULL DEFAULT 0
        COMMENT '箱子宽度cm',
    box_height DECIMAL(10,2) NOT NULL DEFAULT 0
        COMMENT '箱子高度cm',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    remark TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_rule_code (rule_code),
    KEY idx_customer_product (customer_product_name),
    KEY idx_customer_spec (customer_specification),
    KEY idx_status (status)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_unicode_ci;

-- ------------------------------------------
-- 组合商品匹配规则明细表
  CREATE TABLE IF NOT EXISTS product_match_rule_items (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    rule_id BIGINT UNSIGNED NOT NULL
        COMMENT '所属组合匹配规则',
    sku_id BIGINT UNSIGNED NOT NULL
        COMMENT '关联单品SKU',
    quantity_per_set DECIMAL(15,3) NOT NULL DEFAULT 1
        COMMENT '每套组合消耗的该SKU数量',
    sort_order INT NOT NULL DEFAULT 0,
    remark VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_rule_sku (rule_id, sku_id),
    KEY idx_sku_id (sku_id),
    CONSTRAINT fk_match_item_rule
        FOREIGN KEY (rule_id)
        REFERENCES product_match_rules(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_match_item_sku
        FOREIGN KEY (sku_id)
        REFERENCES skus(id)
        ON DELETE RESTRICT
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_unicode_ci;

-- -------------------------------------------
-- 订单行匹配结果表
  CREATE TABLE IF NOT EXISTS order_item_matches (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    order_item_id BIGINT UNSIGNED NOT NULL
        COMMENT '客户原始订单商品行ID',

    sku_id BIGINT UNSIGNED NOT NULL
        COMMENT '匹配到的单品SKU ID',

    rule_id BIGINT UNSIGNED DEFAULT NULL
        COMMENT '使用的组合匹配规则ID，单品直配时为空',

    quantity_per_set DECIMAL(15,3) NOT NULL DEFAULT 1
        COMMENT '每套组合包含该SKU的数量',

    ordered_quantity DECIMAL(15,3) NOT NULL DEFAULT 0
        COMMENT '客户订单原始数量',

    required_quantity DECIMAL(15,3) NOT NULL DEFAULT 0
        COMMENT '该SKU的实际需求数量',

    sku_code_snapshot VARCHAR(100) NOT NULL DEFAULT '',
    product_name_snapshot VARCHAR(255) NOT NULL DEFAULT '',
    model_snapshot VARCHAR(255) NOT NULL DEFAULT '',
    color_snapshot VARCHAR(100) NOT NULL DEFAULT '',

    rule_code_snapshot VARCHAR(100) NOT NULL DEFAULT ''
        COMMENT '匹配时的规则编号快照',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    KEY idx_order_item_id (order_item_id),
    KEY idx_sku_id (sku_id),
    KEY idx_rule_id (rule_id),

    CONSTRAINT fk_order_item_matches_item
        FOREIGN KEY (order_item_id)
        REFERENCES order_items(id)
        ON DELETE CASCADE,

    CONSTRAINT fk_order_item_matches_sku
        FOREIGN KEY (sku_id)
        REFERENCES skus(id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_order_item_matches_rule
        FOREIGN KEY (rule_id)
        REFERENCES product_match_rules(id)
        ON DELETE SET NULL

) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_unicode_ci;


-- ========================================
-- PI 生成扩展：产品简称/美金售价、材料、发货、库存、形式发票
-- 详见 docs/PI生成方案.md
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
        COMMENT 'PI 美金售价(成本价保留在 price)' AFTER price,
    ADD COLUMN declare_price DECIMAL(15,4) NOT NULL DEFAULT 0
        COMMENT '报关申报单价(USD)，0 表示按报关品类的默认申报价' AFTER price_usd;

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
    carryover_amount DECIMAL(15,2) NOT NULL DEFAULT 0
        COMMENT '历史欠款(上期结余)',
    total_unpaid   DECIMAL(15,2) NOT NULL DEFAULT 0
        COMMENT '累计未收 = 本期合计 + 历史欠款 − 已收款',
    paid_amount    DECIMAL(15,2) NOT NULL DEFAULT 0
        COMMENT '已收款',
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


-- ========================================
-- 采购订单（持久化，可人工维护，可超量采购 → 入库形成库存）
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


-- ========================================
-- 生产订单（持久化）+ 产品用料清单(BOM)
-- ========================================

CREATE TABLE IF NOT EXISTS production_orders (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    po_number       VARCHAR(50) NOT NULL DEFAULT '',
    source_order_id BIGINT UNSIGNED DEFAULT NULL,
    source_purchase_order_id BIGINT UNSIGNED DEFAULT NULL
        COMMENT '来源采购订单(从采购下推时写入)',
    plan_date       DATE NULL,
    start_date      DATE NULL,
    finish_date     DATE NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'draft'
        COMMENT 'draft/producing/completed/cancelled',
    work_type       VARCHAR(20) NOT NULL DEFAULT 'produce'
        COMMENT 'produce(投料生产)/pack(包装装箱)/none(只记录)',
    remark          TEXT,
    created_by      VARCHAR(100) NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_prod_number (po_number),
    KEY idx_source_order (source_order_id),
    KEY idx_source_purchase (source_purchase_order_id),
    KEY idx_work_type (work_type),
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


-- ========================================
-- 收款 + 系统设置
-- ========================================

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
    ('low_stock_threshold', '0', '低库存阈值(库存 <= 该值时在统计中列出)'),
    ('exchange_rate', '7.1', '默认人民币汇率(1 外币 = N CNY)，未维护月度汇率的月份使用');


-- ========================================
-- SKU 排序
-- 详见 sql/upgrade_sku_sort_order.sql（已存在的库增量执行用）
-- ========================================

-- ----------------------------------------
-- 产品(SKU)排序值：产品列表统一按它升序展示
-- ----------------------------------------
ALTER TABLE skus
    ADD COLUMN sort_order INT NOT NULL DEFAULT 0
        COMMENT '产品排序值，产品列表按升序展示(越小越靠前)' AFTER id,
    ADD KEY idx_sku_sort_order (sort_order);


-- ========================================
-- 操作日志
-- 详见 sql/upgrade_operation_log.sql（已存在的库增量执行用）
-- ========================================

CREATE TABLE IF NOT EXISTS operation_logs (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    module        VARCHAR(50)  NOT NULL DEFAULT ''
        COMMENT '模块标识，如 orders',
    module_name   VARCHAR(50)  NOT NULL DEFAULT ''
        COMMENT '模块中文名，如 订单管理',
    action        VARCHAR(20)  NOT NULL DEFAULT ''
        COMMENT '动作标识：create/update/delete/execute/export/other',
    action_name   VARCHAR(20)  NOT NULL DEFAULT ''
        COMMENT '动作中文名，如 新增',
    description   VARCHAR(255) NOT NULL DEFAULT ''
        COMMENT '操作摘要，如 新增采购订单',
    method        VARCHAR(10)  NOT NULL DEFAULT ''
        COMMENT '请求方法',
    path          VARCHAR(255) NOT NULL DEFAULT ''
        COMMENT '接口路由(如 /api/orders/:id)',
    uri           VARCHAR(500) NOT NULL DEFAULT ''
        COMMENT '实际请求地址(含查询串)',
    target_id     VARCHAR(100) NOT NULL DEFAULT ''
        COMMENT '操作对象主键(可空)',
    operator      VARCHAR(100) NOT NULL DEFAULT ''
        COMMENT '操作人',
    ip            VARCHAR(64)  NOT NULL DEFAULT '',
    status        INT          NOT NULL DEFAULT 0
        COMMENT 'HTTP 状态码',
    success       TINYINT(1)   NOT NULL DEFAULT 1,
    error_message VARCHAR(500) NOT NULL DEFAULT '',
    duration_ms   BIGINT       NOT NULL DEFAULT 0,
    detail        TEXT
        COMMENT '请求参数摘要(JSON，已截断)',
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_module (module),
    KEY idx_action (action),
    KEY idx_success (success),
    KEY idx_operator (operator),
    KEY idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='关键操作日志';