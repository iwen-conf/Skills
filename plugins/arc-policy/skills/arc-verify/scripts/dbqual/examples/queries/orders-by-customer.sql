SELECT id, customer_id, status, amount_cents, created_at
FROM sqlverify_demo.orders
WHERE customer_id = $1
  AND created_at >= $2::timestamptz
ORDER BY created_at DESC, id DESC
LIMIT $3;
