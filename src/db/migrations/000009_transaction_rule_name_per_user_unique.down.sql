ALTER TABLE transaction_rules DROP CONSTRAINT IF EXISTS transaction_rules_user_name_unique;

ALTER TABLE transaction_rules ADD CONSTRAINT transaction_rules_name_key UNIQUE (name);
