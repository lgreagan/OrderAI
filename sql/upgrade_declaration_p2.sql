-- ========================================
-- 出口报关资料 P2 待补项：增量升级脚本
-- 目标库：order_ai
-- 内容：① SKU 的亚马逊标识（ASIN/FNSKU，报关模板「产品ASIN/产品FNSKU」列）
--       ② 报关资料导出留档表 declaration_records（每次导出存快照，便于复现）
-- 执行方式：
--   mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_declaration_p2.sql
-- 说明：与 sql/upgrade_declaration.sql 同为一次性增量脚本，重复执行会因列/表已存在而报错。
-- ========================================

-- ----------------------------------------
-- SKU 的 ASIN / FNSKU
-- 报关模板 DE 的 S/T 列（产品ASIN* / 产品FNSKU*）按品类去重拼接本表取值。
-- ----------------------------------------
ALTER TABLE skus
    ADD COLUMN asin VARCHAR(50) NOT NULL DEFAULT ''
        COMMENT '亚马逊 ASIN(报关模板产品ASIN列)' AFTER category,
    ADD COLUMN fnsku VARCHAR(50) NOT NULL DEFAULT ''
        COMMENT '亚马逊 FNSKU(报关模板产品FNSKU列)' AFTER asin,
    ADD KEY idx_sku_asin (asin);

-- ----------------------------------------
-- 报关资料导出留档（每次导出存一份当时的快照）
-- ----------------------------------------
CREATE TABLE IF NOT EXISTS declaration_records (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  shipment_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '发货单 ID',
  template_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '导出时使用的报关模板',
  template_name VARCHAR(100) NOT NULL DEFAULT '',
  order_no      VARCHAR(100) NOT NULL DEFAULT '',
  file_name     VARCHAR(300) NOT NULL DEFAULT '' COMMENT '下载文件名',
  line_count    INT NOT NULL DEFAULT 0 COMMENT '明细行数',
  carton_sum    DECIMAL(15,3) NOT NULL DEFAULT 0 COMMENT '总箱数',
  doc           JSON NULL COMMENT '导出时的报关资料快照(DeclarationDoc)',
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_dr_shipment (shipment_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='报关资料导出留档';
