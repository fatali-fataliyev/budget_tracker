CREATE TABLE IF NOT EXISTS `expense_category` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `max_amount` DECIMAL(20, 2),
    `period_day` INT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `note` VARCHAR(1000),
    `created_by` CHAR(36) NOT NULL
);

ALTER TABLE `expense_category`
ADD CONSTRAINT fk_created_by_expense_category
FOREIGN KEY (`created_by`) 
REFERENCES `user` (`id`)
ON DELETE CASCADE;


CREATE UNIQUE INDEX unique_expense_category_per_user ON `expense_category`(`name`, `created_by`);