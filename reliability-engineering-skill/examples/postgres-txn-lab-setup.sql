DROP TABLE IF EXISTS txn_lab_accounts;
CREATE TABLE txn_lab_accounts (
    id bigint PRIMARY KEY,
    balance bigint NOT NULL
);
INSERT INTO txn_lab_accounts(id, balance) VALUES (1, 1000), (2, 1000);
