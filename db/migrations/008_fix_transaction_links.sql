UPDATE `transaction` t
JOIN `income_category` ic ON t.category_name = ic.name AND t.created_by = ic.created_by
SET t.category_id = ic.id
WHERE t.category_type = '+';

UPDATE `transaction` t
JOIN `expense_category` ec ON t.category_name = ec.name AND t.created_by = ec.created_by
SET t.category_id = ec.id
WHERE t.category_type = '-';

ALTER TABLE `transaction` DROP COLUMN `category_name`;