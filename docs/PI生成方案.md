# PI（形式发票）生成方案 —— 产品 / 材料 / 发货 / 费用

> 目标：在现有 OrderAI 上补齐「产品、材料、发货、费用（含库存）」四个环节，最终根据系统真实数据一键生成与 `excel/PI.xlsx` 对齐的 PI（Excel），**系统里没有的数据一律留空**，由业务员在 Excel 里补填。
>
> 本版依据反馈收敛：**不建 PI 抬头/客户信息主数据表**；分组抬头直接取订单上已有的 `order_number / shipping_method / customer` 三个字段。

---

## 1. 本版修订说明（相对初版的取舍）

| 项 | 初版 | 本版（按反馈）|
| --- | --- | --- |
| 我方抬头 `company_profile` | 建表 | ❌ **不建**，Excel 抬头留空 |
| 客户 `customers` | 建表 | ❌ **不建**，用 `orders.customer` 文本；买方块留空 |
| 供应商 `suppliers` | 建表 | ❌ **不建**，材料表里用 `supplier_name` 文本字段即可 |
| 收款 `payments` / 历史欠款 `carryovers` | 建表 | ❌ **暂不建**，欠款行在 Excel 留空，业务员填（P2 可选加回）|
| 产品 | 扩展 `skus` | ✅ 保留，重点是**产品简称**等 PI 字段 |
| 材料 | `materials` | ✅ **重点**（包材/原材料，人工维护）|
| 发货 | `shipments`+`items` | ✅ **重点**（分批发货、运费、保险）|
| 费用 | 分散在多表 | ✅ **重点**，统一到发货费用 + 费用行两张表 |
| PI 单据 | 建 6 张表 | ✅ 精简为 2 张：`proforma_invoices` + `proforma_invoice_items`（其余用留空/文本）|
| 抬头/客户字段 | 系统填 | ✅ 只填 `order_number / shipping_method / customer`，其余留空 |

**结果：新增表从 18 张降到 11 张 + 1 视图，改造 1 张（`skus`）。**

---

## 2. 现状盘点（简要）

| 项 | 现状 |
| --- | --- |
| 技术栈 | Go + Echo v4 + MySQL(order_ai) + Go template + Bootstrap/jQuery，端口 `:8080` |
| 现有表（实测 7 张）| `orders` `order_items` `ai_results` `skus` `product_match_rules` `product_match_rule_items` `order_item_matches` |
| 现有"采购/生产订单" | **不是表**，是每次请求实时算出的**只读视图**（`PurchaseOrderService`/`ProductionOrderService`），按钮只改 `orders.status` |
| 现有缺口 | 无库存、无发货、无材料、无费用、无 PI；采购/生产不可维护、不可入库 |

### 2.1 现有可复用字段（PI 抬头就用这三个）

`orders`：`order_number`（单号）、`shipping_method`（运输方式）、`customer`（客户姓名）——**PI 的抬头/分组只用这三个，不额外建表。**

---

## 3. PI.xlsx 结构拆解 + 留空策略

已完整解析 `excel/PI.xlsx`（Sheet1，A1:H41）。逐块处理：

| PI 区块 | 系统处理 | 说明 |
| --- | --- | --- |
| 抬头横幅（公司名/地址/网址/邮箱/Logo）| **留空** | 业务员在 Excel 里填/贴 |
| `PI No.` / `Issued date` | **自动填** | 规则 `PI-YYYYMMDD[-NN]` |
| `Buyer` 块（Company/Address/Contact/E-mail/Tel）| **留空**（Company 可填 `orders.customer`）| 不建客户表 |
| `Seller` 块 | **留空** | 不建抬头表 |
| 分组标题 `UK142 (18ctns) ship by fastest truck` | **自动填** | `{order_number} ({箱数}ctns) {shipping_method}` |
| 明细行 `V1-8cm bluex64pcs (64pcs/ctn, 1ctn)` \| 单价 \| 数量 \| 金额 | **自动填**（来自发货明细 + 产品简称）| 见 §6.3 |
| 分组小计 `Quantity/Item Cost` | **自动算** | 分组数量/金额合计 |
| `Box orders`（箱费行）| **自动填**（来自材料）；明细里没有箱费行时，编辑界面默认补一行 `box cost`（数量 = 总箱数，单价可改，金额 = 数量 × 单价 只读）。**纸箱采购价是人民币，生成时按签发月份汇率折算成美元单价**（2026-10-18）| `materials` 采购/印刷 |
| `Box and item in Stock` 文本块 | **自动草稿 + 留空可改** | 由库存汇总生成，缺的留空 |
| `Insurance Cost` / `Shipping cost ...` | **自动填**（来自发货单费用）| 见 §4.4 |
| `Amount` | **自动算** | 货款+箱费+保险+运费 |
| `Carryover (previous unpaid)` | **自动算** | 历史欠款 = 该客户历史 PI 合计 − 历史收款（负数按 0）；为 0 时不输出 |
| `Paid Amount` | **自动算** | 本单已收款（来自收款记录）；为 0 时不输出 |
| `Total unpaid Amount` | **自动算** | 累计未收 = `Amount` + 历史欠款 − 已收款；**始终输出**，作为汇总的最后一行，让客户一眼看到还有多少钱未付 |

---

## 4. 领域模型与表结构

```
产品(产品+简称)          材料(包材/原材料)        发货                       费用
└ skus(扩展)             └ materials             ├ shipments                ├ shipments.freight_cost
                                                  └ shipment_items           ├ shipments.insurance_cost
库存                    采购                       (运费/保险在这)             └ proforma_invoice_charges
└ inventory_              ├ purchase_orders                                     (保险/运费/其他费用行)
  transactions           └ purchase_order_items
  + v_inventory_stock
                         生产
                         ├ production_orders
PI                       ├ production_order_items
├ proforma_invoices      └ product_bom(产品用料)
└ proforma_invoice_items
```

**新增 11 张表 + 1 视图 + 改造 1 张表**（`skus`）。

### 4.1 产品（扩展 `skus`，重点是简称）

```sql
ALTER TABLE skus
  ADD COLUMN short_name VARCHAR(100) NOT NULL DEFAULT '' COMMENT 'PI 产品简称,如 V1-8cm blue' AFTER product_name_zh,
  ADD COLUMN series     VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '系列/版本 V1/V2' AFTER short_name,
  ADD COLUMN size_spec  VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '尺寸 6cm/7cm/8cm' AFTER series,
  ADD COLUMN pi_color   VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '客户称呼的颜色 blue/green/orange' AFTER size_spec,
  ADD COLUMN unit       VARCHAR(30)  NOT NULL DEFAULT 'pcs' AFTER pack_quantity,
  ADD COLUMN hs_code    VARCHAR(50)  NOT NULL DEFAULT '' COMMENT '报关编码' AFTER unit,
  ADD COLUMN category   VARCHAR(30)  NOT NULL DEFAULT 'finished' COMMENT 'finished/packaging/material';
```

> **为什么必须加**：PI 里的 `V1-8cm blue` 在系统里叫 `V1 Treat Dispenser Ball Light Blue 8 cm`（薄荷绿）——`short_name` 就是消除这个人工翻译的关键字段。

### 4.2 材料（包材 / 原材料，人工维护）

```sql
CREATE TABLE materials (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code          VARCHAR(100) NOT NULL DEFAULT '',
  name          VARCHAR(255) NOT NULL DEFAULT '',      -- 英文名
  name_zh       VARCHAR(255) NOT NULL DEFAULT '',
  short_name    VARCHAR(100) NOT NULL DEFAULT '',      -- PI 简称,如 "V1 7cm box"
  category      VARCHAR(30)  NOT NULL DEFAULT 'packaging', -- packaging(包材) / material(原材料)
  spec          VARCHAR(255) NOT NULL DEFAULT '',
  unit          VARCHAR(30)  NOT NULL DEFAULT 'pcs',
  price         DECIMAL(15,2) NOT NULL DEFAULT 0,      -- PI 箱费单价(0.11)
  currency      VARCHAR(20)  NOT NULL DEFAULT 'CNY',
  supplier_name VARCHAR(255) NOT NULL DEFAULT '',      -- 供应商直接填文本
  safety_stock  DECIMAL(15,3) NOT NULL DEFAULT 0,
  status        VARCHAR(20)  NOT NULL DEFAULT 'active',
  remark        TEXT,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_material_code (code),
  KEY idx_category (category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 4.3 发货（分批发货 + 运费 + 保险）—— 重点

```sql
CREATE TABLE shipments (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  shipment_number VARCHAR(50) NOT NULL DEFAULT '',
  order_id        BIGINT UNSIGNED NOT NULL,             -- 客户订单(取单号/运输方式/客户)
  batch_no        INT NOT NULL DEFAULT 1,               -- 第几批
  ship_date       DATE NULL,
  shipping_method VARCHAR(100) NOT NULL DEFAULT '',     -- fastest truck / sea ...
  carrier         VARCHAR(100) NOT NULL DEFAULT '',
  tracking_no     VARCHAR(100) NOT NULL DEFAULT '',
  carton_count    DECIMAL(15,3) NOT NULL DEFAULT 0,     -- 本批总箱数
  freight_cost    DECIMAL(15,2) NOT NULL DEFAULT 0,     -- 运费(费用)
  insurance_cost  DECIMAL(15,2) NOT NULL DEFAULT 0,     -- 保险(费用)
  currency        VARCHAR(20) NOT NULL DEFAULT 'USD',
  status          VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft/shipped/delivered
  remark          TEXT,
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_shipment_number (shipment_number),
  KEY idx_order (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE shipment_items (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  shipment_id   BIGINT UNSIGNED NOT NULL,
  order_item_id BIGINT UNSIGNED DEFAULT NULL,          -- 关联客户订单行 → 累计已发货量
  item_type     VARCHAR(20) NOT NULL DEFAULT 'sku',    -- sku(产品) / material(材料/箱子)
  sku_id        BIGINT UNSIGNED DEFAULT NULL,
  material_id   BIGINT UNSIGNED DEFAULT NULL,
  short_name    VARCHAR(100) NOT NULL DEFAULT '',      -- PI 简称
  description   VARCHAR(500) NOT NULL DEFAULT '',      -- PI 描述整串
  quantity      DECIMAL(15,3) NOT NULL DEFAULT 0,      -- 本批发货数量
  unit          VARCHAR(30)  NOT NULL DEFAULT '',
  pack_quantity DECIMAL(15,3) NOT NULL DEFAULT 0,      -- 每箱数
  carton_count  DECIMAL(15,3) NOT NULL DEFAULT 0,
  source        VARCHAR(20) NOT NULL DEFAULT 'stock',  -- stock(库存) / production(新生产)
  unit_price    DECIMAL(15,4) NOT NULL DEFAULT 0,
  amount        DECIMAL(15,2) NOT NULL DEFAULT 0,
  is_paid       TINYINT NOT NULL DEFAULT 0,            -- PI 里显示 "Paid"
  remark        VARCHAR(500) NOT NULL DEFAULT '',
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_shipment (shipment_id),
  KEY idx_order_item (order_item_id),
  CONSTRAINT fk_si_shipment FOREIGN KEY (shipment_id)
    REFERENCES shipments(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

> **已发货量** = `SUM(shipment_items.quantity) WHERE order_item_id=?`；**剩余待发** = 需求 − 已发货量。运费/保险直接挂在发货单上，就是 PI 的 `Shipping cost ... / Insurance Cost` 行。

### 4.4 费用

费用分两处承载，不单独再建主表：

1. **发货费用**：`shipments.freight_cost` / `shipments.insurance_cost`（每批一段）→ PI 的运费/保险行。
2. **其它费用行**：`proforma_invoice_charges`（见 §4.6）承载保险、运费、以及"其它"的手工费用行。
3. **箱费**：来自 `materials.price（人民币/箱） ÷ 汇率 × 数量`（见 §6.3），落到 PI 的 `Box orders` 段；
   汇率取 PI 签发月份的月度汇率（`exchange_rate_yyyy-MM`），该月未维护时用默认汇率 `exchange_rate`。

### 4.5 库存台账（支撑"当前库存 / 已发货量"）

```sql
CREATE TABLE inventory_transactions (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  item_type   VARCHAR(20) NOT NULL DEFAULT 'sku',    -- sku / material
  item_id     BIGINT UNSIGNED NOT NULL,
  direction   TINYINT NOT NULL DEFAULT 1,            -- 1=入库, -1=出库
  biz_type    VARCHAR(30) NOT NULL DEFAULT '',       -- purchase_in/production_in/shipment_out/consumption/adjust
  quantity    DECIMAL(15,3) NOT NULL DEFAULT 0,      -- 绝对值
  unit_cost   DECIMAL(15,4) NOT NULL DEFAULT 0,
  currency    VARCHAR(20) NOT NULL DEFAULT 'CNY',
  warehouse   VARCHAR(50) NOT NULL DEFAULT 'default',
  batch_no    VARCHAR(100) NOT NULL DEFAULT '',
  ref_type    VARCHAR(30) NOT NULL DEFAULT '',       -- purchase_order/production_order/shipment/manual
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
SELECT item_type, item_id, warehouse, SUM(direction * quantity) AS stock_qty
FROM inventory_transactions
GROUP BY item_type, item_id, warehouse;
```

### 4.6 采购订单（持久化、可人工维护、可超量 → 形成库存）

```sql
CREATE TABLE purchase_orders (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  po_number      VARCHAR(50) NOT NULL DEFAULT '',
  supplier_name  VARCHAR(255) NOT NULL DEFAULT '',
  source_order_id BIGINT UNSIGNED DEFAULT NULL,
  order_date     DATE NULL,
  expected_date  DATE NULL,
  currency       VARCHAR(20) NOT NULL DEFAULT 'CNY',
  status         VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft/ordered/partial/received/cancelled
  goods_amount   DECIMAL(15,2) NOT NULL DEFAULT 0,
  freight_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
  total_amount   DECIMAL(15,2) NOT NULL DEFAULT 0,
  remark         TEXT,
  created_by     VARCHAR(100) NOT NULL DEFAULT '',
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_po_number (po_number),
  KEY idx_source_order (source_order_id),
  KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE purchase_order_items (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  purchase_order_id BIGINT UNSIGNED NOT NULL,
  item_type         VARCHAR(20) NOT NULL DEFAULT 'sku',  -- sku / material
  item_id           BIGINT UNSIGNED DEFAULT NULL,
  item_code         VARCHAR(100) NOT NULL DEFAULT '',
  item_name         VARCHAR(255) NOT NULL DEFAULT '',
  short_name        VARCHAR(100) NOT NULL DEFAULT '',
  spec              VARCHAR(255) NOT NULL DEFAULT '',
  unit              VARCHAR(30)  NOT NULL DEFAULT '',
  required_quantity DECIMAL(15,3) NOT NULL DEFAULT 0,    -- 系统建议(客户需求)
  quantity          DECIMAL(15,3) NOT NULL DEFAULT 0,    -- 实际采购(可 > 建议,凑折扣)
  unit_price        DECIMAL(15,4) NOT NULL DEFAULT 0,
  amount            DECIMAL(15,2) NOT NULL DEFAULT 0,
  received_quantity DECIMAL(15,3) NOT NULL DEFAULT 0,    -- 已入库
  remark            VARCHAR(500) NOT NULL DEFAULT '',
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_po (purchase_order_id),
  CONSTRAINT fk_poi_po FOREIGN KEY (purchase_order_id)
    REFERENCES purchase_orders(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 4.7 生产订单 + 产品用料（BOM）

```sql
CREATE TABLE production_orders (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  po_number       VARCHAR(50) NOT NULL DEFAULT '',
  source_order_id BIGINT UNSIGNED DEFAULT NULL,
  plan_date       DATE NULL,
  start_date      DATE NULL,
  finish_date     DATE NULL,
  status          VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft/producing/completed/cancelled
  remark          TEXT,
  created_by      VARCHAR(100) NOT NULL DEFAULT '',
  created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_prod_number (po_number),
  KEY idx_source_order (source_order_id),
  KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE production_order_items (
  id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  production_order_id BIGINT UNSIGNED NOT NULL,
  product_type        VARCHAR(20) NOT NULL DEFAULT 'rule',  -- rule(组合套) / sku(单品)
  product_ref_id      BIGINT UNSIGNED DEFAULT NULL,         -- rule_id 或 sku_id
  product_name        VARCHAR(255) NOT NULL DEFAULT '',
  short_name          VARCHAR(100) NOT NULL DEFAULT '',
  spec                VARCHAR(255) NOT NULL DEFAULT '',
  planned_quantity    DECIMAL(15,3) NOT NULL DEFAULT 0,     -- 计划生产(套/个)
  required_quantity   DECIMAL(15,3) NOT NULL DEFAULT 0,     -- 客户需求(系统建议)
  produced_quantity   DECIMAL(15,3) NOT NULL DEFAULT 0,     -- 已生产
  sets_per_carton     DECIMAL(15,3) NOT NULL DEFAULT 0,
  carton_count        DECIMAL(15,3) NOT NULL DEFAULT 0,
  unit                VARCHAR(30)  NOT NULL DEFAULT '',
  remark              VARCHAR(500) NOT NULL DEFAULT '',
  created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_prod (production_order_id),
  CONSTRAINT fk_poi_prod FOREIGN KEY (production_order_id)
    REFERENCES production_orders(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 产品用料清单：某产品(规则/单品)每单位需要哪些材料
CREATE TABLE product_bom (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  product_type      VARCHAR(20) NOT NULL DEFAULT 'rule',   -- rule / sku
  product_ref_id    BIGINT UNSIGNED NOT NULL,
  material_id       BIGINT UNSIGNED NOT NULL,
  quantity_per_unit DECIMAL(15,4) NOT NULL DEFAULT 1,
  unit              VARCHAR(30) NOT NULL DEFAULT '',
  remark            VARCHAR(500) NOT NULL DEFAULT '',
  status            VARCHAR(20) NOT NULL DEFAULT 'active',
  created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_bom (product_type, product_ref_id, material_id),
  KEY idx_material (material_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 4.8 PI 单据（精简 2 张：头 + 明细）

```sql
CREATE TABLE proforma_invoices (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  pi_number      VARCHAR(50) NOT NULL DEFAULT '',
  customer_name  VARCHAR(255) NOT NULL DEFAULT '',   -- 取 orders.customer 文本
  issued_date    DATE NULL,
  currency       VARCHAR(20) NOT NULL DEFAULT 'USD',
  status         VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft/issued/closed
  item_amount    DECIMAL(15,2) NOT NULL DEFAULT 0,   -- 货款
  box_amount     DECIMAL(15,2) NOT NULL DEFAULT 0,   -- 箱费
  insurance_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
  freight_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
  amount         DECIMAL(15,2) NOT NULL DEFAULT 0,   -- 本期合计
  stock_note     TEXT,                               -- "Box and item in Stock" 文本块(可留空)
  remark         TEXT,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_pi_number (pi_number),
  KEY idx_issued (issued_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- PI 明细/分组/费用行：用 item_type 区分，一张表搞定(避免多张表)
CREATE TABLE proforma_invoice_items (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  pi_id        BIGINT UNSIGNED NOT NULL,
  item_type    VARCHAR(20) NOT NULL DEFAULT 'goods',  -- group(分组标题) / goods(货款) / box(箱费) / charge(费用) / note(备注文本)
  group_no     INT NOT NULL DEFAULT 0,                -- 第几组
  group_title  VARCHAR(255) NOT NULL DEFAULT '',      -- 分组标题: {单号} ({箱数}ctns) {运输方式}
  order_id     BIGINT UNSIGNED DEFAULT NULL,
  shipment_id  BIGINT UNSIGNED DEFAULT NULL,
  sku_id       BIGINT UNSIGNED DEFAULT NULL,
  material_id  BIGINT UNSIGNED DEFAULT NULL,
  model        VARCHAR(100) NOT NULL DEFAULT '',      -- PI 的 Model 列
  short_name   VARCHAR(100) NOT NULL DEFAULT '',
  description  VARCHAR(500) NOT NULL DEFAULT '',      -- PI 描述整串
  quantity     DECIMAL(15,3) NOT NULL DEFAULT 0,
  unit         VARCHAR(30) NOT NULL DEFAULT 'pcs',
  unit_price   DECIMAL(15,4) NOT NULL DEFAULT 0,
  amount       DECIMAL(15,2) NOT NULL DEFAULT 0,
  is_paid      TINYINT NOT NULL DEFAULT 0,            -- 显示 "Paid"
  charge_type  VARCHAR(20) NOT NULL DEFAULT '',       -- insurance/shipping/other(仅 item_type=charge)
  title        VARCHAR(255) NOT NULL DEFAULT '',      -- 费用行标题文本
  sort_order   INT NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_pi (pi_id),
  CONSTRAINT fk_pii_pi FOREIGN KEY (pi_id)
    REFERENCES proforma_invoices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**状态机**：`purchase_orders: draft→ordered→partial→received`；`production_orders: draft→producing→completed`；`shipments: draft→shipped→delivered`；`proforma_invoices: draft→issued→closed`。

**库存联动**：采购 `received`/手工录入 → `+purchase_in`/`+manual_in`；生产 `completed` → 成品 `+production_in`（材料不再自动扣减，改由「领料出库」页面按实际用量写 `-requisition`，可按损耗多领）；发货 `shipped` → `-shipment_out`。

---

## 5. 关键业务流程

### 5.0 单据下推（订单 → 采购单 / 生产单 / 发货单）
```
客户订单 → "从订单下推" → 选择订单 → 下推并保存
  采购单：按 SKU 需求下推并保存【原始需求快照】(required_quantity 与 quantity 均写需求/建议量，
          单价默认取 SKU 成本价 skus.price)，status=draft → 采购员改数量/单价
  生产单：按展开的组合套/单品下推并保存【原始需求快照】(required_quantity 与 planned_quantity)，
          status=draft；同时按装箱信息下推纸箱用料（见下）
  发货单：按该订单【剩余未发数量】下推一张草稿，默认全部一票发完
          (批次号/运输方式取自订单，箱数 = 数量 ÷ 每箱装)，status=draft → 发货员拆分批次或改数量
  同一订单若已有下推草稿 → 直接复用该草稿，避免重复下推
```
下推后自动打开草稿供人工维护；订单侧的原始需求始终保留在 `required_quantity`。

> 注意：发货单的"已发/剩余"统计**包含草稿**（`shipment_items` 不看状态），所以草稿会先占住订单数量；
> 这正是下推复用草稿的原因。要分批发货时，把草稿数量改小 → 剩余量自动释放 → 再新建第二批。

**纸箱用料下推**（生产单）：
```
组合套取 product_match_rules 箱规、单品取 skus 箱规
  → 自动建/复用纸箱材料（编码 CTN-{rule_code|sku_code}，类别 packaging，
     规格 = 长×宽×高，如 40×30×25cm）
  → 写入 BOM：产品 → 纸箱，用量 = 1 / 每箱数量
完工时：已生产数量 × 用量 = 领用箱数（如 80 套 ÷ 64/箱 = 1.25 箱）
```

### 5.1 采购（可超量 → 库存）
```
采购订单 → "从订单下推"（保存需求快照） 或 选中客户订单 → "载入建议"
  建议量 = Σ需求(order_item_matches.required_quantity) − 当前库存 − 在途未入库
  人工可上调(凑折扣)、加材料行 → 保存 → 下单 → 到货登记 → 入库(+库存)
```
超出客户需求的部分自动沉淀为库存。

### 5.2 生产（含材料）
```
生产订单 → "从订单下推"（保存需求快照 + 纸箱用料） 或 载入建议后人工维护计划量
  → 按 product_bom 带出用料（含下推的纸箱） → 完工 → 成品入库(+库存) + 材料领料(-库存)
```

### 5.3 发货（可分批）
```
新建发货单 → 选订单行 → 填本批发货量(可部分) + 来源(库存/新生产)
  + 运输方式/箱数/运费/保险费 → 保存 → 发货 → 扣库存 + 累计已发货量
```
一张订单可多张发货单（第 1 批、第 2 批…），对应 PI 的多个分组。

### 5.4 PI 生成（核心）
```
选客户 + 时间范围(或勾选发货单/订单) → "生成 PI"
  ① 分组：每张发货单 → 一组
       标题 = "{order_number} ({ctns}ctns) {shipping_method}"
  ② 货款行：发货明细 → description = "{product_name}x{qty}pcs ({pack}pcs/ctn, {ctns}ctn)"
            product_name = order_items.product_name + specification（英文，见 §6.3）
            price = sku.price_usd（PI 美金售价），发货单价为 0 时兜底取主数据
            amount = price×qty；is_paid → "Paid"
  ③ 箱费行：本期材料(包材)采购/印刷 → "Box orders"；明细里没有箱费行时，编辑界面默认补一行 "box cost"
            （数量 = 分组标题箱数合计，单价可编辑，金额 = 数量 × 单价，不可编辑）
            单价 = 材料人民币价 ÷ 汇率（签发月份月度汇率，缺省用默认汇率），保留 2 位小数
  ④ 费用行：每张发货单 → 运费 "Shipping cost by fast truck for {单号} for {ctns}ctns"、保险
  ⑤ 库存说明：v_inventory_stock 自动生成 left/used/will leave 草稿 → stock_note(可留空/改)
  ⑥ 合计：货款+箱费+保险+运费 = amount
  (欠款两行留空)
→ 人工校对(描述/价格/Paid/库存文本，缺的留空) → 保存 → 导出 Excel
```

**描述模板**：`{product_name}x{quantity}pcs ({pack_quantity}pcs/ctn, {cartons}ctn)`
例：`V1 Light Blue 6 cmx288pcs (144pcs/ctn, 2ctn)`

`{product_name}` 由 `order_items.product_name + specification` 生成（英文）：剔除 `Treat Dispenser Ball`、相邻重复词去重（`V2 V2 Orange 6 cm` → `V2 Orange 6 cm`），详见 §6.3。

**PI 单号**：`PI-YYYYMMDD`，同日多张 → `PI-YYYYMMDD-NN`。

---

## 6. PI 生成功能设计

### 6.1 页面

| 路由 | 页面 | 优先级 |
| --- | --- | --- |
| `/proforma-invoices` | PI 列表 + 新建 | P0 |
| `/proforma-invoices/:id` | PI 详情/编辑 + 导出 Excel | P0 |
| `/materials` | 材料（包材/原材料）维护 | P1 |
| `/shipments` | 发货管理 | P1 |
| `/inventory` | 库存台账（结存+流水+盘点）| P1 |
| `/purchase-orders` | 采购订单（改造为持久化）| P1 |
| `/production-orders` | 生产订单（改造为持久化）| P1 |
| 产品页 | 增加"产品简称"等字段 | P0 |

### 6.2 API（示例）
```
POST /api/proforma-invoices/generate       按客户+范围预览(不落库)
POST /api/proforma-invoices                保存(头+明细)
GET  /api/proforma-invoices/:id            详情
PUT  /api/proforma-invoices/:id            更新
GET  /api/proforma-invoices/:id/export     导出 Excel(模板填充, 缺失留空)
POST /api/shipments  /api/shipments/:id    发货 CRUD
GET  /api/inventory  POST /api/inventory/adjust
GET  /api/materials  POST /api/materials
GET  /api/purchase-orders   /api/production-orders   (持久化 CRUD)
```

### 6.3 Excel 导出（直接克隆 PI.xlsx 模板）
- 引入 `github.com/xuri/excelize/v2`（纯 Go）；`PiExcelService.Build` **直接以 `excel/PI.xlsx` 为模板**：`excelize.OpenFile` 打开后克隆它的样式、列宽、行高与合并单元格。
- **抬头保留**：只保留模板第 1..12 行（公司抬头/Logo、`PI No.`、签发日期、买卖方标签、明细表头 `Model / Description / Price / Qty.(pcs) / Amount(US$)`）；模板自身贴在明细区的样例截图与示例数据（值/合并/超链接）在导出时清除。
- **Buyer / Seller 取自模板**：买方与卖方整块（`A6:H11`）沿用 `PI.xlsx` 里已填好的资料，不再留空；只在系统有值时覆盖 `H4` PI 号、`H5` 签发日期、`B7` 买方公司名。
- **产品图片**：货款行把 `skus.image` 的图片插到产品名前（Model 列 A），按行高等比缩放并居中，图片文件缺失时自动跳过。
- **产品名**：货款行 Description = `order_items.product_name + specification` 生成的英文名 + 数量/装箱信息，例：`V1 Light Blue 6 cmx288pcs (144pcs/ctn, 2ctn)`。
  - 剔除 `Treat Dispenser Ball`；相邻重复词去重：`V2 Treat Dispenser Ball V2 Orange 6 cm` → `V2 Orange 6 cm`。
  - 解析入口 `ProformaInvoiceService.GetForExport`（按 `order_id + sku_id` 经 `order_item_matches` 反查 `order_items`）；箱费行取材料编码 `CTN-<sku_code>` 对应的英文名。
- **整份 PI 不含中文**：导出前统一过滤中日韩文字/全角标点；库存说明（含中文时）用英文产品名重建，抬头取系统设置、设置项含中文时回退 `Current Stock`。
- 打印沿用现有 `#printWrap` + `window.print()` 方案（与生产订单一致）。

---

## 7. 与现有代码的集成改造清单

| 层 | 文件（新增/改造）|
| --- | --- |
| 表 | 追加 §4 DDL 到 `sql/init.sql`（或新增 `sql/upgrade_pi.sql` 增量脚本）|
| models | 新增 `Material.go` `Shipment.go` `Inventory.go` `ProformaInvoice.go`；改造 `sku.go`（加字段）`PurchaseOrder.go` `ProductionOrder.go`（改为持久化结构）|
| repository | 新增 `MaterialRepository` `ShipmentRepository` `InventoryRepository` `ProformaInvoiceRepository`；`PurchaseOrderRepository` `ProductionOrderRepository` 重写为持久化|
| services | 新增 `MaterialService` `ShipmentService` `InventoryService` `ProformaInvoiceService`（含装配算法）`PiExcelService`（导出）；采购/生产 Service 改造|
| controllers | 上述对应 Controller（沿用 `{"success","data","message"}` 约定）|
| routes | `routes.go` 注册新页面/API；`layout.html` 加导航（材料/发货/库存/PI）|
| templates | 新增 `proforma_invoices.html` `materials.html` `shipments.html` `inventory.html`；改造 `products.html`（采购/生产页在 P1 改造）|
| 依赖 | `go.mod` 增加 `github.com/xuri/excelize/v2` |

---

## 8. 分期实施计划

**P0 — 最小可用 PI**
1. `skus` 加简称等字段 + 产品页维护。
2. `materials` 材料表 + 维护页。
3. `proforma_invoices` + `proforma_invoice_items` 两表。
4. **PI 生成**：选客户/订单 → 用 `order_item_matches` 装配明细（无库存/发货版）→ 编辑/保存/导出 Excel（缺的留空）。

**P1 — 履约闭环（产品/材料/发货/库存/采购/生产）**
5. `shipments` + `shipment_items` 发货（分批、运费、保险）。
6. `inventory_transactions` + 视图 + 库存页。
7. `purchase_orders(+items)` 持久化 + 入库。
8. `production_orders(+items)` + `product_bom` + 完工入库/领料。
9. PI 装配升级：分组=发货单、已发货量、箱费、运费/保险。

**P2 — 可选**
10. 收款/历史欠款（`payments`、carryover 自动累计）——若不做，Excel 留空即可。
11. 统计看板；顺带补上导航里已存在但未实现的 `/statistics`、`/settings`。

---

## 9. 已确认决策（原待确认项，均已拍板/核实）

| # | 事项 | 结论 |
| --- | --- | --- |
| 1 | **报价币种** | ✅ **新增 `skus.price_usd` 作为 PI 美金售价**，原 `price` 保留为成本价（CNY），两者分开。 |
| 2 | **材料范围** | ✅ `materials` **同时覆盖包材（`packaging`）与原材料（`material`）**，用 `category` 区分。 |
| 3 | **箱费来源** | ✅ PI 的 `Box orders` **由本期包材采购/消耗自动带出**，无数据则留空由业务员手填。 |
| 4 | **PI 分组粒度** | ✅ **一发货单 = 一组**（贴合分批发货，每组带箱数/运输方式）。 |
| 5 | **`Paid` 行** | ✅ 仅展示，金额**不计入合计**。已用样例验证：`125.44+915.84+668.8 = 1710.08`。 |
| 6 | **数据库名** | ✅ 已核实实际库为 `order_ai`（`orderai` 不存在）→ 修正 `sql/init.sql`。 |
| 7 | **Go 版本** | ✅ 已核实 `go build ./...` 成功（toolchain go1.26.8），无需改动。 |

---

## 10. 实现进度（已完成）

> 已建成并通过端到端验证（`go build` / `go vet` 通过，页面与 API 实测正常）。

| 模块 | 交付 | 状态 |
| --- | --- | --- |
| 数据库 | `sql/init.sql` 修正库名 + 追加新表；新增 `sql/upgrade_pi.sql` 增量脚本（已应用到 `order_ai`）| ✅ |
| 产品 | `skus` 新增 `short_name/series/size_spec/pi_color/unit/hs_code/category/price_usd`；产品页支持维护 | ✅ |
| 材料 | `materials` 表 + 模型/仓储/服务/控制器 + `templates/materials.html` | ✅ |
| 库存 | `inventory_transactions` 台账 + `v_inventory_stock` 视图 + 库存页（结存/流水/盘点调整）| ✅ |
| 发货 | `shipments` + `shipment_items` + 发货页；保存时按状态自动扣减库存 | ✅ |
| PI | `proforma_invoices` + `proforma_invoice_items`；按订单/发货单自动装配、编辑、保存、删除 | ✅ |
| Excel 导出 | `excelize/v2` 生成 PI 工作簿，缺失资料留空（`GET /api/proforma-invoices/:id/export`）| ✅ |
| 采购订单持久化 | `purchase_orders` + `purchase_order_items`；采购单 CRUD、按订单生成建议（需求 − 库存 − 在途）、到货登记 → `purchase_in` 入库；原只读视图改为 `PurchaseDemand*` 保留为"建议"数据源 | ✅ |
| 箱费自动带出 | PI 生成时把**关联订单已到货的材料采购**自动落为 `Box orders` 行 | ✅ |
| 生产订单持久化 | `production_orders` + `production_order_items`；生产单 CRUD、按订单生成建议（组合套按"套"/单品按"个"）、完工登记 → 成品 `production_in` 入库（材料改由「领料出库」页面 `requisition` 出库）| ✅ |
| 产品用料(BOM) | `product_bom` + 维护页；领料时按 `用量 × 计划量` 带出需求量参考（实际领用可含损耗）| ✅ |
| 收款/欠款 | `payments` + PI 的 `carryover_amount/total_unpaid/paid_amount`；按客户自动累计历史欠款（历史 PI 合计 − 历史收款，收款超过开票额时按 0 计）；**累计未收 = 本期合计 + 历史欠款 − 本单已收款**，收款登记/修改/删除后自动回写 PI 的已收款与累计未收 | ✅ |
| 库存说明草稿 | PI 生成/刷新时按 `v_inventory_stock` 自动生成 `stock_note`，抬头可配置 | ✅ |
| 数据统计 | `/statistics`：订单/采购/生产/发货/PI/收款汇总 + 负库存 / 低库存预警 | ✅ |
| 系统设置 | `app_settings` + `/settings`：默认币种/仓库、PI 条款与有效期、库存说明抬头、低库存阈值 + 运行时信息 | ✅ |
| 单据下推 | 客户订单 → 采购单 / 生产单**下推并保存原始需求快照**（同一订单已有草稿则复用）；采购单价默认关联 **SKU 成本价**（`skus.price`，可改）；生产下推同时按 SKU/组合规则装箱信息自动下推**纸箱用料**（材料 + BOM，用量 = 1/每箱）| ✅ |
| 发货下推 | 客户订单 → 发货单**下推**（`POST /api/shipments/order/:orderId/push`）：按该订单**剩余未发数量**一次性生成一张发货单草稿，**默认全部一票发完**（批次/运输方式取自订单，毛重箱数按每箱装自动算）；同一订单已有草稿则复用，明细行提供「全部发完」一键填充 | ✅ |
| 配置 | `APP_PORT` 生效（原先 `.env` 里有但未使用）| ✅ |
| 单据打印 | `templates/purchase_order_print.html` / `production_order_print.html`：独立 A4 打印页（`/purchase-orders/:id/print`、`/production-orders/:id/print`），含抬头/状态/元信息栏、统计条、明细表、合计行、备注与打印时间；一键 `打印 / 保存为 PDF` | ✅ |
| 单据截图 | `templates/purchase_order_screen.html` / `production_order_screen.html`：**对外（工厂/第三方）精简版**（`/purchase-orders/:id/screen`、`/production-orders/:id/screen`），列严格对齐参考件（`docs/采购表.png`、`docs/生产订单 - OrderAI2026.pdf`），不出现完整 SKU 码：采购 = 序号/产品中文名/型号/颜色/采购数量/金额，生产 = 行/产品名/简码/数量/每箱装/箱数；简码取编码后三位 | ✅ |
| 汇总报表 | `templates/purchase_report.html` / `production_report.html`：`/purchase-orders/report`、`/production-orders/report`，按状态/关键字筛选；采购侧给出采购单汇总（货款/运费/合计/到货行）+ **物品采购汇总**（客户需求/采购量/已入库/未到货/金额）；生产侧给出生产单汇总（箱数/完工行）+ **用料汇总**（材料/规格/需求数量/涉及产品）| ✅ |

**新增/变更文件**：`sql/{init,upgrade_pi,upgrade_purchase,upgrade_production,upgrade_accounts}.sql`、`models/{sku,date,Material,Inventory,Shipment,ProformaInvoice,PurchaseDemand,PurchaseOrder,ProductionDemand,ProductionOrder,ProductBom,Payment,AppSetting,Statistics}.go`、`repository/{Material,Inventory,Shipment,ProformaInvoice,PurchaseDemand,PurchaseOrder,ProductionDemand,ProductionOrder,ProductBom,Payment,AppSetting,Statistics}*.go`、`services/{helpers,Material,Inventory,Shipment,ProformaInvoice,PiExcel,PurchaseOrder,ProductionOrder,ProductBom,Payment,Settings,Statistics}*.go`、`controllers/{material,inventory,shipment,proforma_invoice,purchase_order,production_order,product_bom,payment,settings,statistics}.go`、`templates/{materials,inventory,material_issues,shipments,proforma_invoices,purchase_orders,production_orders,product_bom,payments,statistics,settings,purchase_order_print,production_order_print,purchase_order_screen,production_order_screen,purchase_report,production_report}.html`、`web/js/products.js`、`routes/routes.go`、`main.go`、`config/config.go`。

### 端到端验证结果
- **PI（订单）**：分组标题 `162 (7ctns) SEA`、货款行取自匹配明细；`64 × 1.96 = 125.44`（与 `PI.xlsx` 样例一致）。
- **PI（发货单）**：分组 `162 (3ctns) fast truck`、运费 `Shipping cost for 162 for 3ctns`(120)、保险(30)，合计 150。
- **PI（箱费）**：材料采购（`V1 7cm box` 40 × 0.11）到货后自动带出 `Box orders` 行 4.4。
- **发货 → 库存**：已发货状态写入 `shipment_out`（SKU −64 / −288）。
- **采购 → 库存**：超量采购到货写 `purchase_in`（+500/+200）；部分到货 `partial`；建议量 `max(0, 需求−库存−在途)`；删除撤销流水；编辑保留已入库量与单号。
- **生产 → 库存**：完工后组合规则按组成单品 `production_in`（+72/+72、+40/+40、+80）；材料不再自动扣减，改由「领料出库」页面按实际用量写 `requisition`（可含损耗）；部分完工 `producing`；删除撤销流水。
- **收款/欠款**：登记收款 50 关联 PI → PI `paid_amount=50`、`total_unpaid` 相应减少 50；同客户新 PI 自动 `carryover = 历史 PI 合计 − 历史收款`（收款多于开票时按 0 计）；统计页 `pi_paid=50 / payment_count=1`。
- **库存说明**：自动草稿输出 `西瓜球8厘米草绿: 100 pcs`、`西瓜球6厘米薄荷绿: 200 pcs` 等真实结存；`refresh-stock-note` 可重算。
- **系统设置**：`default_warehouse` 保存后回读为 `main`（默认仓库已接入采购/生产/发货入库出库）。
- **下推**：订单 8 → 采购单（4 行 SKU，`required_quantity=quantity=需求`，**单价取 SKU 成本价** 4.65/6.12/9.60/9.60，货款 4161.60，状态 `draft`）；重复下推返回同一草稿（不重复创建）。订单 17 → 生产单（3 个组合套）+ 自动创建 3 个纸箱材料（`CTN-B08Y946Q4R` 规格 `40×30×25cm` 等）+ 3 条 BOM（用量 `1/72`、`1/40`、`1/64`）；完工后纸箱按箱数领料（−1 / −1 / −1.25，与"套数 ÷ 每箱"一致）。
- **页面/接口**：`/materials` `/shipments` `/inventory` `/proforma-invoices` `/payments` `/statistics` `/settings` `/products` `/purchase-orders` `/production-orders` `/product-bom` 均 200。

### 打印 / 报表验证结果
- **采购单打印**：`/purchase-orders/16/print` 200，渲染 `PO-20261011-03`（状态徽标、供应商/来源订单/下单日期/预计到货/币种元信息栏、10 列表格、合计行、金额口径说明、备注、打印时间）。
- **生产单打印**：`/production-orders/5/print` 200，`MO-20261011` 渲染 10 行明细（需求/计划量/每箱/箱数/已生产/单位，合计箱数 51）+ **用料清单**（产品/材料编码/材料名称/**规格**/单位用量/需求数量/单位），规格取自 `materials.spec`（`35×35×35cm`、`46×31×42cm` 等纸箱尺寸），单位用量按 6 位小数显示（`1/64 = 0.015625`、`1/144 = 0.006944`）。
- **采购报表**：`/purchase-orders/report` 200，采购单数 3 / 明细行数 10 / 货款合计 54144.32 CNY；物品采购汇总合计与 `SUM(amount)` 一致。
- **生产报表**：`/production-orders/report` 200，1 张生产单 / 10 明细行 / 合计箱数 51 / 材料项数 10；用料汇总的 10 个纸箱材料按 `计划量 × 单位用量` 汇总（与单张打印页逐行一致）。
- **筛选**：`?status=`、`?keyword=` 同时作用于单据列表与汇总（`status=draft` 的生产用料汇总返回 0 行、`keyword=nomatch` 返回 0 行，页面切换到「已完工」后正常回显）。
- **回归**：新增聚合查询未改动既有 SQL 语义；`go build` / `go vet` 均通过。

### 单据截图验证结果（对照参考件逐列核对）
- **生产截图** `/production-orders/5/screen` 200，与 `docs/生产订单 - OrderAI2026.pdf` **逐行一致**：
  - 标题 `生产订单 162`（取来源订单号，与参考件同）、元信息栏 `客户 Polat / 国家 UK / 运输方式 SEA / 订单时间 2026-10-10 23:21`、统计条 `产品行数 10 行 / 组合规则 0 行 / 单品直配 10 行 / 合计箱数 51 箱`
  - 列 `行 / 产品名 / 简码 / 数量 / 每箱装 / 箱数`，10 行数值全对（`33P 64 个 64 个 1`、`MWP 288 个 144 个 2`、`252 160 个 80 个 2`、`MMY 128 个 64 个 2`、`LJ7 64 个 64 个 1`、`XCK 1008 个 144 个 7`、`B8F 576 个 64 个 9`、`1NW 144 个 144 个 1`、`JQX 64 个 64 个 1`、`TF1 2000 个 80 个 25`），合计 `51`
  - 简码 = 产品编码后三位；`pcs` 翻译为 `个`（与参考件的 `个` 一致）；产品名下的灰色小字为英文名，**不再显示完整 SKU 码**
- **采购截图** `/purchase-orders/14/screen` 200，与 `docs/采购表.png` 结构一致：
  - 标题 `采购批次汇总 PO-20261011`、元信息栏 `客户/国家/运输方式/订单时间`、列 `序号 / 产品中文名 / 型号 / 颜色 / 采购数量 / 金额`、合计行 `合计（9 种单品）| 2496 | 18144.32 CNY`（与参考件 `合计 (9 种单品) 2576 18921.76 CNY` 排版一致）
  - 型号取 `skus.model`（回退明细快照 `spec`）、颜色取 `skus.color`（回退按型号前缀从产品名里截出），实测 `专利球6厘米橙色 / 专利球6厘米 / 橙色 / 144 / 783.36` 与参考件同款数据一致
  - **材料行**（纸箱）实测：产品中文名 = 纸箱名、型号 = 纸箱规格（`46×31×42cm`）、颜色留空；合计文案按类型区分（`1 种材料` / `9 种单品`），不会把纸箱叫成"单品"
- **布局**：产品名列吸收剩余宽度（`th.wide { width:100% }`），在 765px 窄窗口下英文名仍单行显示、无横向溢出，适合整屏截图。

### 发货下推验证结果
- **接口**：`POST /api/shipments/order/18/push` → `SH-20261011-01`，批次 1、发货日期 2026-10-11、运输方式 SEA（取自订单）、状态 `draft`、币种 USD、合计箱数 51；10 行明细数量 = 该订单剩余未发量（64/288/160/128/64/1008/576/144/64/2000），每箱装与箱数（1/2/2/2/1/7/9/1/1/25）由 `数量 ÷ 每箱装` 自动算出，描述 = `{产品名}x{qty}pcs ({pack}pcs/ctn, {ctn}ctn)`。
- **不重复下推**：再次 `push` 同一订单 → `created=false`，返回同一张草稿，发货单总数仍为 1（草稿的明细会占用订单"已发数量"，重复创建会导致重复计发）。
- **库存**：草稿状态不写库存流水（`inventory_transactions` 为 0）；改为「已发货」才按明细 `shipment_out` 出库（沿用既有逻辑）。
- **页面**：发货管理页新增「从订单下推」按钮 + 订单选择弹窗；下推成功后自动刷新列表并打开该草稿的编辑弹窗（订单/批次/发货日期/运输方式/备注/10 行勾选与数量均已带出）。明细表新增「全部发完」按钮：新建单（剩余=客户需求）与编辑草稿（剩余已含本单）两种场景都按"本单最多可发 = 剩余 + 本单已填"一键填充，实测 5 行/10 行均正确。
- **测试数据已清理**：测试用发货单（2、3 号）已删除，`shipments`/`shipment_items`/`inventory_transactions` 均回到 0。

## 11. 遗留与后续

方案范围内的功能已全部实现。可选的后续增强：

| 项 | 说明 |
| --- | --- |
| PI 打印视图 | 目前导出 Excel；如需 PDF/打印版可参照采购/生产单打印页（独立 A4 模板 + `window.print()`）补一个 `pi_print.html` |
| 生产完工数量登记 | `MO-20261011` 状态已是「已完工」，但明细的 `produced_quantity` 仍为 0（未走「完工」登记的保存），故打印页/报表的「已生产/完工行」显示 0；在「完工」弹窗里填实际数量并保存后即会回显 |
| 报表导出 | 报表页目前用浏览器「打印 / 保存为 PDF」；如需 Excel 导出可复用 `services/PiExcelService.go`（excelize） |
| 截图金额列 | 采购截图的「金额」列按参考件保留；若不想让工厂看到金额，去掉 `purchase_order_screen.html` 里那一列即可（或做成开关） |
| 截图接口 | 目前是单张单据；如需"按客户订单汇总一次截图"（参考件 `采购批次汇总 161` 那种口径）可加一个按 order 聚合的版本 |
| 报表日期区间 | 当前筛选按状态 + 关键字（服务端），如需按下单/计划日期区间过滤可加 `from/to` 参数 |
| 发货"已发"口径 | `ListOrderLines` 的已发/剩余**包含草稿**（不看发货单状态）。若希望只有「已发货/已送达」才算已发（草稿不占数量），可把统计子查询加上 `JOIN shipments ... AND status IN ('shipped','delivered')` |
| 发货单价 | 发货明细单价取自 `skus.price_usd`；生成 PI 时若明细单价为 0（如发货单早于售价维护），按 `skus.price_usd`（货款）/ `materials.price`（箱费）兜底，因此 PI 商品金额 = 数量 × 单价；产品页仍未填美金售价的产品，PI 金额才会是 0 |
| 收款对账 | 目前按"客户级收款 + PI 级已收"，如需按 PI 分摊/核销可加核销明细 |
| 权限与操作日志 | 当前无登录/角色，`created_by/operator` 字段已预留 || 多仓库 | `inventory_transactions.warehouse` 已支持，界面暂按默认仓库展示 |
