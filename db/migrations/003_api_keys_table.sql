CREATE TABLE IF NOT EXISTS api_keys (
    owner_id CHAR(36) PRIMARY KEY NOT NULL,
    `key` VARCHAR(255) UNIQUE NOT NULL,
    created_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) CHARACTER SET utf8 COLLATE utf8_general_ci;