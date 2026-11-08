CREATE TABLE IF NOT EXISTS inventory (
    product_id  TEXT PRIMARY KEY,
    stock       INT NOT NULL CHECK (stock >= 0),
    reserved    INT NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
