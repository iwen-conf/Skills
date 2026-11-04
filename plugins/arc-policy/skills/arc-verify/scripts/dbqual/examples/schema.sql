CREATE SCHEMA IF NOT EXISTS sqlverify_demo;

DROP TABLE IF EXISTS sqlverify_demo.orders;
DROP TABLE IF EXISTS sqlverify_demo.customers;
DROP TABLE IF EXISTS sqlverify_demo.accounts;

CREATE TABLE sqlverify_demo.customers (
  id bigint PRIMARY KEY,
  segment text NOT NULL,
  last_seen_at timestamptz NOT NULL
);

CREATE TABLE sqlverify_demo.orders (
  id bigint PRIMARY KEY,
  customer_id bigint NOT NULL REFERENCES sqlverify_demo.customers(id),
  status text NOT NULL,
  amount_cents bigint NOT NULL,
  created_at timestamptz NOT NULL
);

CREATE INDEX orders_customer_created_idx
  ON sqlverify_demo.orders(customer_id, created_at DESC, id DESC);

CREATE TABLE sqlverify_demo.accounts (
  id bigint PRIMARY KEY,
  balance bigint NOT NULL CHECK (balance >= 0)
);
