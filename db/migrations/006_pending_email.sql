USE budget_tracker;

-- Drop column email_verified if it exists
SET
    @col_exists := (
        SELECT
            COUNT(*)
        FROM
            information_schema.COLUMNS
        WHERE
            TABLE_SCHEMA = DATABASE()
            AND TABLE_NAME = 'users'
            AND COLUMN_NAME = 'email_verified'
    );

SET
    @drop_stmt := IF(
        @col_exists > 0,
        'ALTER TABLE users DROP COLUMN email_verified;',
        'SELECT \"Column email_verified does not exist.\"'
    );

PREPARE stmt
FROM
    @drop_stmt;

EXECUTE stmt;

DEALLOCATE PREPARE stmt;

-- Add column pending_email if it doesn't exist
SET
    @pending_exists := (
        SELECT
            COUNT(*)
        FROM
            information_schema.COLUMNS
        WHERE
            TABLE_SCHEMA = DATABASE()
            AND TABLE_NAME = 'users'
            AND COLUMN_NAME = 'pending_email'
    );

SET
    @add_stmt := IF(
        @pending_exists = 0,
        'ALTER TABLE users ADD COLUMN pending_email VARCHAR(255) NOT NULL;',
        'SELECT \"Column pending_email already exists.\"'
    );

PREPARE stmt2
FROM
    @add_stmt;

EXECUTE stmt2;

DEALLOCATE PREPARE stmt2;