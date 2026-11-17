"""Adapter contract tests only; these are not pay_gateway product E2E receipts."""
import copy
import importlib.util
from pathlib import Path
import sqlite3
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location("pay_arc_adapter", Path(__file__).with_name("adapter.py"))
adapter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(adapter)


def sample():
    session = {"caller_id": "caller-one", "scope": "owned-case-one", "payment_no": "case_payment",
               "refund_no": "case_refund", "identity": {"database_name": "qa_owned_one"},
               "database_binding": {"database": "qa_owned_one", "role": "test_owner", "database_oid": 100}}
    payment = {"id": "payment-1", "kind": "payment", "caller_id": "caller-one", "parent_order_id": None,
               "merchant_order_no": "case_payment", "channel_account_id": adapter.ACCOUNT,
               "provider": "huiju", "provider_request_id": "provider-1", "amount": 100, "currency": "CNY"}
    database = {**session["database_binding"], "caller_enabled": True, "account_enabled": True,
                "orders": [payment], "payment_dispatches": 1, "refund_dispatches": 0,
                "payment_confirmed": False, "refund_notifications": 0, "refund_callbacks": 0}
    observation = {"database": {"name": "qa_owned_one", "oid": 100},
                   "provider_observation": {"errors": []},
                   "provider_receipts": [{"operation": "CreatePayment", "provider": "huiju", "request_id": "provider-1"}]}
    catalog = {"channel_accounts": [{"id": adapter.ACCOUNT, "provider": "huiju", "supported_operations": ["refund"]}]}
    return session, database, observation, catalog


class PayAdapterTests(unittest.TestCase):
    def test_indexed_payment_is_not_a_paid_payment(self):
        facts = adapter.normalize(*sample())
        self.assertEqual(facts["state"], "PAYMENT_INDEXED")
        self.assertIs(facts["values"]["provider.payment_confirmed"], False)
        self.assertNotIn("request.amount", facts["values"])
        self.assertNotIn("transport.status_code", facts["values"])

    def test_cross_account_and_database_identity_are_refused(self):
        for defect in ("caller", "database_oid", "database_name"):
            with self.subTest(defect=defect):
                session, database, observation, catalog = sample()
                if defect == "caller":
                    database["orders"][0]["caller_id"] = "someone-else"
                elif defect == "database_oid":
                    database["database_oid"] = 101
                else:
                    observation["database"]["name"] = "qa_other"
                with self.assertRaises(adapter.Blocked):
                    adapter.normalize(session, database, observation, catalog)

    def test_wrong_persisted_business_fields_remain_visible_for_contract_failure(self):
        session, database, observation, catalog = sample()
        database["orders"][0].update(amount=999, currency="USD", channel_account_id="wrong-account")
        facts = adapter.normalize(session, database, observation, catalog)
        self.assertEqual(facts["values"]["payment.amount"], 999)
        self.assertEqual(facts["values"]["payment.currency"], "USD")
        self.assertEqual(facts["values"]["payment.channel_account_id"], "wrong-account")

    def test_missing_and_corrupt_institution_receipts_do_not_mean_zero(self):
        for defect in ("missing_receipts", "missing_errors", "actual_error"):
            session, database, observation, catalog = sample()
            if defect == "missing_receipts":
                del observation["provider_receipts"]
            elif defect == "missing_errors":
                del observation["provider_observation"]["errors"]
            else:
                observation["provider_observation"]["errors"] = ["identity mismatch"]
            with self.subTest(defect=defect), self.assertRaises(adapter.Blocked):
                adapter.normalize(session, database, observation, catalog)

    def test_counts_and_wrong_relationship_remain_actual_facts(self):
        session, database, observation, catalog = sample()
        refund = copy.deepcopy(database["orders"][0])
        refund.update(id="refund-1", kind="refund", parent_order_id="wrong-parent",
                      merchant_order_no="case_refund", provider_request_id="refund-request-1")
        database["orders"].append(refund)
        database["refund_dispatches"] = 2
        observation["provider_receipts"] += [{"operation": "CreateRefund", "provider": "huiju", "request_id": "refund-request-1"}] * 2
        facts = adapter.normalize(session, database, observation, catalog)
        self.assertEqual(facts["state"], "REFUND_INDEXED")
        self.assertFalse(facts["values"]["relation.parent_valid"])
        self.assertEqual(facts["values"]["provider.refund_dispatches"], 2)
        self.assertEqual(facts["values"]["provider.refund_received"], 2)

    def test_orphan_outbounds_are_visible_without_any_order_index(self):
        session, database, observation, catalog = sample()
        database.update(orders=[], payment_dispatches=1, refund_dispatches=1)
        observation["provider_receipts"] = [
            {"operation": "CreatePayment", "provider": "huiju", "request_id": "orphan-payment"},
            {"operation": "CreateRefund", "provider": "huiju", "request_id": "orphan-refund"},
        ]
        facts = adapter.normalize(session, database, observation, catalog)
        self.assertEqual(facts["state"], "EMPTY")
        self.assertEqual(facts["values"]["provider.payment_dispatches"], 1)
        self.assertEqual(facts["values"]["provider.refund_dispatches"], 1)
        self.assertEqual(facts["values"]["provider.payment_received"], 1)
        self.assertEqual(facts["values"]["provider.refund_received"], 1)
        self.assertEqual(facts["values"]["provider.unexpected_receipts"], 2)

    def test_wrong_provider_and_unrelated_creation_receipts_cannot_prove_delivery(self):
        for defect in ("wrong_provider", "wrong_request", "wrong_operation"):
            with self.subTest(defect=defect):
                session, database, observation, catalog = sample()
                receipt = observation["provider_receipts"][0]
                if defect == "wrong_provider":
                    receipt["provider"] = "yeepay"
                elif defect == "wrong_request":
                    receipt["request_id"] = "unrelated-request"
                else:
                    receipt["operation"] = "CreateRefund"
                facts = adapter.normalize(session, database, observation, catalog)
                self.assertEqual(facts["values"]["provider.unexpected_receipts"], 1)
                self.assertEqual(facts["values"]["provider.payment_received"] +
                                 facts["values"]["provider.refund_received"], 1)

    def test_query_receipts_do_not_inflate_creation_counts(self):
        session, database, observation, catalog = sample()
        observation["provider_receipts"] += [
            {"operation": operation, "provider": "yeepay", "request_id": "query-only"}
            for operation in ("QueryPayment", "QueryRefund", "CancelPayment")
        ]
        facts = adapter.normalize(session, database, observation, catalog)
        self.assertEqual(facts["values"]["provider.payment_received"], 1)
        self.assertEqual(facts["values"]["provider.refund_received"], 0)
        self.assertEqual(facts["values"]["provider.unexpected_receipts"], 0)

    def test_actual_dispatch_count_queries_include_orphans_but_not_queries(self):
        session, _, _, _ = sample()
        with patch.object(adapter, "sql_json", return_value={}) as query:
            adapter.database_snapshot(session)
        statement = query.call_args.args[1]
        # These two COUNT subqueries use portable SQL. Execute the expressions
        # actually sent to PostgreSQL against synthetic rows, not a mock count.
        with sqlite3.connect(":memory:") as database:
            database.execute("CREATE TABLE selected (id TEXT, kind TEXT)")
            database.execute("CREATE TABLE p_provider_dispatches (operation TEXT, resource_id TEXT)")
            database.executemany("INSERT INTO p_provider_dispatches VALUES (?,?)", [
                ("create_payment", "orphan-payment"), ("submit_refund", "orphan-refund"),
                ("query_payment", "orphan-payment"), ("query_refund", "orphan-refund"),
            ])
            for key in ("payment_dispatches", "refund_dispatches"):
                start = statement.index("'" + key + "',(") + len(key) + 4
                depth, end = 1, start
                while depth:
                    depth += (statement[end] == "(") - (statement[end] == ")")
                    end += 1
                count = database.execute(statement[start:end - 1]).fetchone()[0]
                self.assertEqual(count, 1, key + " hid a real dispatch without a selected order")

    def test_duplicate_rows_do_not_choose_a_fictional_unique_entity(self):
        session, database, observation, catalog = sample()
        second = copy.deepcopy(database["orders"][0])
        second.update(id="payment-2", amount=200)
        database["orders"].append(second)
        facts = adapter.normalize(session, database, observation, catalog)
        self.assertEqual(facts["values"]["payment.rows"], 2)
        self.assertTrue(facts["values"]["payment.exists"])
        self.assertEqual(facts["payment_id"], "")
        self.assertIsNone(facts["values"]["payment.amount"])
        self.assertEqual(len(facts["evidence"]["orders"]), 2)

    def test_orphan_refund_remains_an_observed_business_violation(self):
        session, database, observation, catalog = sample()
        database["orders"][0].update(kind="refund", id="refund-orphan", parent_order_id="missing-payment",
                                     merchant_order_no="case_refund")
        facts = adapter.normalize(session, database, observation, catalog)
        self.assertEqual(facts["state"], "REFUND_INDEXED")
        self.assertEqual(facts["values"]["payment.rows"], 0)
        self.assertEqual(facts["values"]["refund.rows"], 1)
        self.assertFalse(facts["values"]["relation.parent_valid"])

    def test_memory_environment_is_not_accepted_as_postgres(self):
        session = {"target": "http://127.0.0.1:23456", "run_id": "owned"}
        value = {"environment": "isolated-browser", "storage": "memory", "provider": "controlled"}
        with patch.object(adapter, "http", return_value=(200, value)), self.assertRaises(adapter.Blocked):
            adapter.identity(session, bind=True)

    def test_only_owned_loopback_fixture_targets_are_supported(self):
        for url in ("https://production.example", "http://127.0.0.1:8080", "http://user:password@localhost:23456", "http://localhost:23456/path"):
            with self.subTest(url=url), self.assertRaises(adapter.Blocked):
                adapter.target_url(url)
        self.assertEqual(adapter.target_url("http://127.0.0.1:23456/"), "http://127.0.0.1:23456")

    def test_sql_oracle_runs_read_only_and_keeps_dsn_out_of_argv(self):
        class Result:
            returncode = 0
            stdout = '{"independently_read":true}'
        session = {"pay_repo": "/candidate", "identity": {"database_name": "qa_owned_one"}}
        config = {"dsn": "postgresql://reader:private-value@db.example/isolated_test"}
        with patch.object(adapter, "isolated_config", return_value=config), \
             patch.object(adapter.shutil, "which", return_value="/usr/bin/psql"), \
             patch.object(adapter.subprocess, "run", return_value=Result()) as called:
            self.assertEqual(adapter.sql_json(session, "SELECT '{}'::json"), {"independently_read": True})
        args, kwargs = called.call_args
        self.assertNotIn("private-value", str(args))
        self.assertIn("BEGIN READ ONLY", kwargs["input"])
        self.assertIn("ROLLBACK", kwargs["input"])
        self.assertTrue(kwargs["env"]["PGDATABASE"].endswith("/qa_owned_one"))


if __name__ == "__main__":
    unittest.main()
