CREATE TABLE IF NOT EXISTS payments (
    id UUID DEFAULT uuid_generate_v4() PRIMARY KEY NOT NULL,
    user_id INTEGER NOT NULL,
    money INTEGER NOT NULL,
    status varchar(10) NOT NULL
)
