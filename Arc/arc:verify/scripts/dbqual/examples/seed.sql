-- Deterministic reference data. The customer distribution is intentionally skewed.
INSERT INTO sqlverify_demo.customers(id, segment, last_seen_at)
SELECT i,
       CASE WHEN i <= 100 THEN 'hot' WHEN i <= 2000 THEN 'medium' ELSE 'cold' END,
       timestamptz '2026-09-01 00:00:00+00' + ((i % 720) || ' hours')::interval
FROM generate_series(1, 10000) AS g(i);

INSERT INTO sqlverify_demo.orders(id, customer_id, status, amount_cents, created_at)
SELECT i,
       CASE
         WHEN i % 10 < 5 THEN 1 + (i % 100)          -- 50% traffic/data on 1% hot customers
         WHEN i % 10 < 8 THEN 101 + (i % 1900)
         ELSE 2001 + (i % 8000)
       END,
       CASE WHEN i % 100 < 60 THEN 'paid'
            WHEN i % 100 < 85 THEN 'completed'
            WHEN i % 100 < 95 THEN 'cancelled'
            ELSE 'pending' END,
       100 + (i % 500000),
       timestamptz '2025-09-01 00:00:00+00' + ((i % 8760) || ' hours')::interval
FROM generate_series(1, 100000) AS g(i);

INSERT INTO sqlverify_demo.accounts(id, balance)
VALUES (1, 100000), (2, 100000), (3, 100000);

ANALYZE sqlverify_demo.customers;
ANALYZE sqlverify_demo.orders;
ANALYZE sqlverify_demo.accounts;
