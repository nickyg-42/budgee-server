-- Remove the old unique constraint (if it exists)
ALTER TABLE budgets DROP CONSTRAINT IF EXISTS budgets_personal_finance_category_key;

-- Add a new unique constraint for (user_id, personal_finance_category)
ALTER TABLE budgets ADD CONSTRAINT budgets_user_category_unique UNIQUE (user_id, personal_finance_category);
