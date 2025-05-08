USE budget_tracker;

CREATE TABLE IF NOT EXISTS sessions (
    id CHAR(36) NOT NULL PRIMARY KEY,
    token VARCHAR(255) NOT NULL UNIQUE,
    created_at DATETIME NOT NULL,
    expire_at DATETIME NOT NULL,
    user_id CHAR(36) NOT NULL
);