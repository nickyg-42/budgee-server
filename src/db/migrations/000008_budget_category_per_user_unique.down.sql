-- Remove the composite unique constraint
ALTER TABLE budgets DROP CONSTRAINT IF EXISTS budgets_user_category_unique;

-- Restore the original unique constraint on personal_finance_category
ALTER TABLE budgets ADD CONSTRAINT budgets_personal_finance_category_key UNIQUE (personal_finance_category);
