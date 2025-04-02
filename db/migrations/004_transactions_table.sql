CREATE TABLE IF NOT EXISTS transactions (
    id CHAR(36) PRIMARY KEY NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,
    currency varchar(3) NOT NULL,
    category ENUM('fun', 'food', 'tickets') NOT NULL,
    created_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    type ENUM('+', '-') NOT NULL,
    created_by CHAR(36) NOT NULL,
    FOREIGN KEY (created_by) REFERENCES users(id)
)