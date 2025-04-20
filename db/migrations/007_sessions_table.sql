USE budget_tracker;

DROP TABLE IF EXISTS api_keys;

CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(36) PRIMARY KEY NOT NULL,
    token VARCHAR(512) NOT NULL,
    created_at DATE NOT NULL,
    expire_at DATE NOT NULL,
    user_id VARCHAR(36) NOT NULL
);