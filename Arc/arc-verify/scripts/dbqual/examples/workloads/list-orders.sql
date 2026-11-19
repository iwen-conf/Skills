\set r random_zipfian(1, 10000, 1.07)
\set customer_id 1 + permute(:r, 10000, :default_seed + 101)
SELECT id, status, amount_cents, created_at
FROM sqlverify_demo.orders
WHERE customer_id = :customer_id
ORDER BY created_at DESC, id DESC
LIMIT 20;
