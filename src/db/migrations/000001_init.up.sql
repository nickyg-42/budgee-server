CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE plaid_items (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    item_id TEXT UNIQUE NOT NULL,
    access_token TEXT UNIQUE NOT NULL,
    institution_id TEXT NOT NULL,
    institution_name TEXT,
    cursor TEXT,
    status text NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT now()
);

CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,
    item_id INTEGER REFERENCES plaid_items(id) ON DELETE CASCADE,
    account_id TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    official_name TEXT,
    mask TEXT NOT NULL,
    type TEXT NOT NULL,
    subtype TEXT NOT NULL,
    current_balance numeric(28,10),
    available_balance numeric(28,10),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT now()
);

CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE,
    transaction_id TEXT UNIQUE NOT NULL,
    plaid_category_id text,
    category text,
    type text NOT NULL,
    name text NOT NULL,
    merchant_name TEXT,
    amount numeric(28,10) NOT NULL,
    currency TEXT,
    date DATE NOT NULL,
    pending BOOLEAN NOT NULL,
    account_owner text,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT now()
);
