CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email TEXT NOT NULL,
  plan TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO users (email, plan) VALUES
  ('ada@example.com', 'enterprise'),
  ('grace@example.com', 'team'),
  ('katherine@example.com', 'team'),
  ('dorothy@example.com', 'starter'),
  ('mary@example.com', 'enterprise');
