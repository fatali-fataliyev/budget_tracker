USE `budget_tracker`;

CREATE TABLE IF NOT EXISTS `user` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `username` VARCHAR(255) NOT NULL UNIQUE,
    `fullname` VARCHAR(255) DEFAULT "unnamed",
    `hashed_password` VARCHAR(255) NOT NULL,
    `email` VARCHAR(255) UNIQUE,
    `pending_email` VARCHAR(255),
    `joined_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);