-- ========================================
-- 操作日志：增量升级脚本
-- 目标库：order_ai
-- 执行：mysql -h 127.0.0.1 -uroot -p order_ai < sql/upgrade_operation_log.sql
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
