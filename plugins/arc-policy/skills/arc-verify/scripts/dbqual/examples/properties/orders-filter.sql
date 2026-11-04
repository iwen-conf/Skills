SELECT id
FROM sqlverify_demo.orders
WHERE customer_id = $1
  AND created_at >= $2::timestamptz
ORDER BY id;
