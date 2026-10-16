CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY,
    name        TEXT           NOT NULL,
    description TEXT           NOT NULL DEFAULT '',
    price       NUMERIC(12, 2) NOT NULL CHECK (price >= 0),
    stock       INTEGER        NOT NULL CHECK (stock >= 0),
    created_at  TIMESTAMPTZ    NOT NULL,
    updated_at  TIMESTAMPTZ    NOT NULL
);

CREATE INDEX IF NOT EXISTS products_created_at_idx ON products (created_at, id);
