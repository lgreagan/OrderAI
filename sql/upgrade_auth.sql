-- ============================================================
-- 登录与账号
--   users          : 账号（密码只存 bcrypt 哈希）
--   user_sessions  : 登录会话（token 服务端校验，重启不掉线）
-- ============================================================

CREATE TABLE IF NOT EXISTS users (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username      VARCHAR(50)     NOT NULL COMMENT '登录名',
    password_hash VARCHAR(255)    NOT NULL COMMENT 'bcrypt 哈希',
    display_name  VARCHAR(50)     NOT NULL DEFAULT '' COMMENT '显示名',
    role          VARCHAR(20)     NOT NULL DEFAULT 'staff' COMMENT 'admin / staff',
    status        VARCHAR(20)     NOT NULL DEFAULT 'active' COMMENT 'active / disabled',
    last_login_at DATETIME        NULL DEFAULT NULL,
    created_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_users_username (username)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '用户账号';

CREATE TABLE IF NOT EXISTS user_sessions (
    token      CHAR(64)        NOT NULL,
    user_id    BIGINT UNSIGNED NOT NULL,
    ip         VARCHAR(64)     NOT NULL DEFAULT '',
    user_agent VARCHAR(255)    NOT NULL DEFAULT '',
    expires_at DATETIME        NOT NULL,
    created_at DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (token),
    KEY idx_sessions_user (user_id),
    KEY idx_sessions_expires (expires_at),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci COMMENT = '登录会话';
