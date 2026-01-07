-- Remove the old unique constraint if it exists
ALTER TABLE transaction_rules DROP CONSTRAINT IF EXISTS transaction_rules_name_key;

-- Add a new unique constraint for (user_id, name)
ALTER TABLE transaction_rules ADD CONSTRAINT transaction_rules_user_name_unique UNIQUE (user_id, name);
