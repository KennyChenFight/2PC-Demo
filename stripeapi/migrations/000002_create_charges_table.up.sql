CREATE TABLE IF NOT EXISTS charges (
    id UUID DEFAULT uuid_generate_v4() PRIMARY KEY NOT NULL,
    user_id INTEGER NOT NULL,
    money INTEGER NOT NULL,
    capture Boolean DEFAULT false NOT NULL
);