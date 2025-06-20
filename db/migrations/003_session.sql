CREATE TABLE IF NOT EXISTS `session` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `token` VARCHAR(255) NOT NULL UNIQUE,
    `created_at` DATETIME NOT NULL,
    `expire_at` DATETIME NOT NULL,
    `user_id` CHAR(36) NOT NULL
);

ALTER TABLE `session`
ADD CONSTRAINT fk_user_session
FOREIGN KEY (`user_id`) 
REFERENCES `user` (`id`)
ON DELETE CASCADE;
