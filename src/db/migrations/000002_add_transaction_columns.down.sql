ALTER TABLE transactions
RENAME COLUMN primary_category TO plaid_category_id;

ALTER TABLE transactions
DROP COLUMN IF EXISTS detailed_category;
ALTER TABLE transactions
DROP COLUMN IF EXISTS payment_channel;
ALTER TABLE transactions
DROP COLUMN IF EXISTS personal_finance_category_icon_url;
