\set order_id random(1, 100000)
SELECT id, customer_id, status, amount_cents, created_at
FROM sqlverify_demo.orders
WHERE id = :order_id;
