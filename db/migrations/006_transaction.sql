CREATE TABLE IF NOT EXISTS `transaction` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `category_name` VARCHAR(255) NOT NULL,
    `category_type` ENUM('+', '-') NOT NULL,
    `amount` DECIMAL(20, 2) NOT NULL,
    `currency` VARCHAR(255) NOT NULL,
    `created_at`DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `note` VARCHAR(1000),
    `created_by` CHAR(36) NOT NULL
);


ALTER TABLE `transaction`
ADD CONSTRAINT fk_created_by_transaction
FOREIGN KEY (`created_by`) 
REFERENCES `user` (`id`)
ON DELETE CASCADE;