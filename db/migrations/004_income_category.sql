USE `budget_tracker`;

CREATE TABLE IF NOT EXISTS `income_category` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `target_amount` DECIMAL(20, 2),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `note` VARCHAR(1000) DEFAULT "",
    `created_by` CHAR(36) NOT NULL
);

ALTER TABLE `income_category`
ADD CONSTRAINT fk_created_by_income_category
FOREIGN KEY (`created_by`) 
REFERENCES `user` (`id`)
ON DELETE CASCADE;

CREATE UNIQUE INDEX unique_income_category_per_user ON `income_category`(`name`, `created_by`);