BEGIN;

ALTER TABLE
    transactions
ADD
    COLUMN new_category varchar(255) not null;

UPDATE
    transactions
SET
    new_category = category;

ALTER TABLE
    transactions DROP COLUMN category;

ALTER TABLE
    transactions RENAME COLUMN new_category TO category;

COMMIT;