-- Treasury Tracker - Seed Data
-- Demo data for development and testing.

-- Users

INSERT INTO users (name, balance, created_at) VALUES
('Dylan Huff', 358825.00, '2023-01-10 10:05:15'),
('Sarah Martinez', 563800.00, '2022-06-01 09:30:00'),
('James Chen', 484350.00, '2024-03-01 12:00:00');

-- Transactions

-- Dylan Huff (User ID: 1)
INSERT INTO transactions (user_id, timestamp, type, term, amount, yield_at_transaction, balance_after) VALUES
(1, '2023-01-10 10:05:15', 'fund', NULL, 400000.00, NULL, 400000.00),
(1, '2023-01-15 11:30:00', 'buy', '6M', 150000.00, 5.10, 250000.00),
(1, '2023-01-20 14:12:45', 'buy', '2Y', 150000.00, 4.90, 100000.00),
(1, '2023-07-25 10:00:21', 'buy', '6M', 150000.00, 5.30, 100000.00),
(1, '2024-02-01 13:00:00', 'withdraw', NULL, 60000.00, NULL, 205150.00),
(1, '2024-02-05 11:45:10', 'buy', '2Y', 100000.00, 4.50, 105150.00),
(1, '2024-08-15 10:20:00', 'sell', '6M', 153825.00, 5.30, 258975.00),
(1, '2024-11-10 09:15:00', 'buy', '1Y', 100000.00, 4.35, 158975.00);

-- Sarah Martinez (User ID: 2)
INSERT INTO transactions (user_id, timestamp, type, term, amount, yield_at_transaction, balance_after) VALUES
(2, '2022-06-01 09:30:00', 'fund', NULL, 700000.00, NULL, 700000.00),
(2, '2022-06-10 10:15:00', 'buy', '2Y', 200000.00, 4.20, 500000.00),
(2, '2022-06-15 15:00:00', 'buy', '30Y', 100000.00, 4.00, 400000.00),
(2, '2023-01-20 11:00:00', 'buy', '6M', 200000.00, 5.20, 200000.00),
(2, '2023-08-01 14:30:15', 'sell', '2Y', 208400.00, 4.20, 408400.00),
(2, '2023-08-15 10:00:00', 'withdraw', NULL, 75000.00, NULL, 333400.00),
(2, '2023-09-01 11:10:00', 'buy', '1Y', 300000.00, 5.40, 33400.00),
(2, '2024-01-15 14:00:00', 'fund', NULL, 100000.00, NULL, 133400.00),
(2, '2024-03-20 10:30:00', 'buy', '10Y', 150000.00, 4.70, 133400.00),
(2, '2024-09-05 09:30:00', 'sell', '1Y', 316200.00, 5.40, 449600.00),
(2, '2024-10-15 11:00:00', 'buy', '5Y', 85800.00, 4.40, 363800.00);

-- James Chen (User ID: 3)
INSERT INTO transactions (user_id, timestamp, type, term, amount, yield_at_transaction, balance_after) VALUES
(3, '2024-03-01 12:00:00', 'fund', NULL, 500000.00, NULL, 500000.00),
(3, '2024-03-05 10:20:00', 'buy', '3M', 200000.00, 5.10, 300000.00),
(3, '2024-03-10 10:25:00', 'buy', '6M', 200000.00, 5.00, 100000.00),
(3, '2024-06-15 11:00:00', 'withdraw', NULL, 25000.00, NULL, 277550.00),
(3, '2024-06-20 14:00:00', 'buy', '3M', 150000.00, 4.80, 127550.00),
(3, '2024-08-20 10:00:00', 'sell', '6M', 204000.00, 5.00, 331550.00),
(3, '2024-10-01 09:30:00', 'buy', '6M', 150000.00, 4.60, 181550.00);

-- Holdings

-- Dylan Huff (User ID: 1)
INSERT INTO holdings (user_id, term, amount, yield_at_purchase, purchase_date, remaining_amount, face_value, purchase_price, security_type) VALUES
(1, '2Y', 150000.00, 4.90, '2023-01-20 14:12:45', 150000.00, 150000.00, 150000.00, 'note'),
(1, '2Y', 100000.00, 4.50, '2024-02-05 11:45:10', 100000.00, 100000.00, 100000.00, 'note'),
(1, '1Y', 100000.00, 4.35, '2024-11-10 09:15:00', 100000.00, 104350.00, 100000.00, 'bill');

-- Sarah Martinez (User ID: 2)
INSERT INTO holdings (user_id, term, amount, yield_at_purchase, purchase_date, remaining_amount, face_value, purchase_price, security_type) VALUES
(2, '30Y', 100000.00, 4.00, '2022-06-15 15:00:00', 100000.00, 100000.00, 100000.00, 'bond'),
(2, '10Y', 150000.00, 4.70, '2024-03-20 10:30:00', 150000.00, 150000.00, 150000.00, 'note'),
(2, '5Y', 85800.00, 4.40, '2024-10-15 11:00:00', 85800.00, 85800.00, 85800.00, 'note');

-- James Chen (User ID: 3)
INSERT INTO holdings (user_id, term, amount, yield_at_purchase, purchase_date, remaining_amount, face_value, purchase_price, security_type) VALUES
(3, '3M', 150000.00, 4.80, '2024-06-20 14:00:00', 150000.00, 151800.00, 150000.00, 'bill'),
(3, '6M', 150000.00, 4.60, '2024-10-01 09:30:00', 150000.00, 153450.00, 150000.00, 'bill');

-- Verification queries (uncomment to test):
-- SELECT name, balance FROM users ORDER BY id;
-- SELECT COUNT(*) as transaction_count, user_id FROM transactions GROUP BY user_id;
-- SELECT COUNT(*) as holdings_count, user_id FROM holdings GROUP BY user_id;
