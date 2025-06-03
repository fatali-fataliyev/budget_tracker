USE budget_tracker;

CREATE TABLE IF NOT EXISTS transactions (
    id CHAR(36) NOT NULL PRIMARY KEY,
    category_name VARCHAR(255) NOT NULL,
    amount DECIMAL(20, 2) NOT NULL,
    -- Negative for expense, positive for income, e.g: -145 or +1500
    currency VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    note VARCHAR(1000),
    created_by CHAR(36) NOT NULL,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);