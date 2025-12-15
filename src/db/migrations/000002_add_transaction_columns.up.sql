ALTER TABLE transactions
RENAME COLUMN plaid_category_id TO primary_category;

ALTER TABLE transactions
ADD COLUMN detailed_category TEXT,
ADD COLUMN payment_channel TEXT,
ADD COLUMN personal_finance_category_icon_url TEXT;

ALTER TABLE transactions
DROP COLUMN IF EXISTS category;