-- FileBox database schema.
-- Run this file once as a MariaDB admin to create the database, tables, and indexes.
-- Example:
--   mariadb -u root -p < scripts/schema.sql
-- The application user and privileges can be created with scripts/db-setup.sh first.

CREATE DATABASE IF NOT EXISTS filebox
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci;

USE filebox;

SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS chunk_uploads;
DROP TABLE IF EXISTS item_shares;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS friend_requests;
DROP TABLE IF EXISTS friends;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS schema_migrations;

CREATE TABLE users (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    email           VARCHAR(255) NOT NULL,
    username        VARCHAR(64) NOT NULL,
    password_hash   CHAR(60) NOT NULL,
    pin_hash        VARCHAR(255) NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'active',
    role            VARCHAR(32) NOT NULL DEFAULT 'user',
    storage_quota   BIGINT UNSIGNED NOT NULL DEFAULT 0,
    total_storage   BIGINT UNSIGNED NOT NULL DEFAULT 0,
    storage_used    BIGINT UNSIGNED NOT NULL DEFAULT 0,
    last_login_at   DATETIME NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at      DATETIME NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_email (email),
    UNIQUE KEY uq_users_username (username),
    KEY idx_users_deleted (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id     BIGINT UNSIGNED NOT NULL,
    token_hash  CHAR(64) NOT NULL,
    user_agent  VARCHAR(255) NULL,
    ip          VARCHAR(45) NULL,
    expires_at  DATETIME NOT NULL,
    revoked     TINYINT(1) NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sessions_token (token_hash),
    KEY idx_sessions_user (user_id),
    KEY idx_sessions_expires (expires_at),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE friends (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id         BIGINT UNSIGNED NOT NULL,
    friend_user_id  BIGINT UNSIGNED NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_friends (user_id, friend_user_id),
    KEY idx_friends_user (user_id),
    KEY idx_friends_friend (friend_user_id),
    CONSTRAINT fk_friends_user        FOREIGN KEY (user_id)        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_friends_friend_user FOREIGN KEY (friend_user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE friend_requests (
    id                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    requester_user_id   BIGINT UNSIGNED NOT NULL,
    recipient_user_id   BIGINT UNSIGNED NOT NULL,
    status              ENUM('pending', 'accepted', 'rejected', 'cancelled') NOT NULL DEFAULT 'pending',
    created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY unique_friend_request (requester_user_id, recipient_user_id),
    KEY idx_recipient_status (recipient_user_id, status),
    KEY idx_requester_status (requester_user_id, status),
    CONSTRAINT fk_fr_requester FOREIGN KEY (requester_user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_fr_recipient FOREIGN KEY (recipient_user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE items (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id         BIGINT UNSIGNED NOT NULL,
    parent_id       BIGINT UNSIGNED NULL,
    type            ENUM('file','folder') NOT NULL,
    name            VARCHAR(255) NOT NULL,
    ext             VARCHAR(32) NULL,
    path            VARCHAR(700) NOT NULL,
    mime            VARCHAR(255) NULL,
    size            BIGINT UNSIGNED NOT NULL DEFAULT 0,
    storage_path    VARCHAR(255) NULL,
    checksum        VARCHAR(255) NULL,
    is_chunked      BOOLEAN NOT NULL DEFAULT FALSE,
    chunk_count     INT NULL,
    chunk_size      INT NULL,
    is_favorite     BOOLEAN NOT NULL DEFAULT FALSE,
    password_hash   VARCHAR(255) NULL,
    encryption_iv   VARCHAR(255) NULL,
    encryption_tag  VARCHAR(255) NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_items_user_parent (user_id, parent_id),
    INDEX idx_items_user_type (user_id, type),
    INDEX idx_items_user_favorite (user_id, is_favorite),
    INDEX idx_items_user_path (user_id, path),
    UNIQUE KEY unique_user_path (user_id, path),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES items(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE item_shares (
    id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    item_id             BIGINT UNSIGNED NOT NULL,
    owner_user_id       BIGINT UNSIGNED NOT NULL,
    shared_with_user_id BIGINT UNSIGNED NOT NULL,
    created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_item_shares (item_id, shared_with_user_id),
    KEY idx_item_shares_item (item_id),
    KEY idx_item_shares_recipient (shared_with_user_id),
    CONSTRAINT fk_item_shares_item             FOREIGN KEY (item_id)             REFERENCES items(id) ON DELETE CASCADE,
    CONSTRAINT fk_item_shares_owner            FOREIGN KEY (owner_user_id)       REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_item_shares_shared_with_user FOREIGN KEY (shared_with_user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE chunk_uploads (
    id              VARCHAR(36) NOT NULL PRIMARY KEY,
    user_id         BIGINT UNSIGNED NOT NULL,
    total_chunks    INT NOT NULL,
    chunk_size      INT NOT NULL,
    metadata        JSON NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at    DATETIME NULL,
    INDEX idx_chunk_uploads_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE api_keys (
    id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id       BIGINT UNSIGNED NOT NULL,
    name          VARCHAR(255) NOT NULL,
    key_hash      CHAR(64) NOT NULL,
    `key`         TEXT NOT NULL DEFAULT '',
    permissions   JSON NULL,
    expires_at    DATETIME NULL,
    last_used_at  DATETIME NULL,
    revoked       TINYINT(1) NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_api_keys_hash (key_hash),
    KEY idx_api_keys_user (user_id),
    CONSTRAINT fk_api_keys_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE settings (
    `key`   VARCHAR(128) NOT NULL,
    `value` TEXT         NULL,
    PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET FOREIGN_KEY_CHECKS = 1;
