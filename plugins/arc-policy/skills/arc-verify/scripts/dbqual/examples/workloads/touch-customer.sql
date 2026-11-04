\set customer_id random(1, 10000)
BEGIN;
UPDATE sqlverify_demo.customers
SET last_seen_at = GREATEST(last_seen_at, clock_timestamp())
WHERE id = :customer_id;
COMMIT;
