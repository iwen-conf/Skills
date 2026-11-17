#!/usr/bin/env python3
"""Run Arc against pay_gateway's owned PostgreSQL browser fixture.

This adapter never starts a database, creates a replacement payment service, or
infers funding status from p_orders. See README.md for the controlled boundaries.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid


REVISION = "e4f054fc7bb06e091be62d6765bb29b4348ad651"
ACCOUNT = "ca_browser_huiju"
ENVIRONMENT = "pay-owned-postgres"
MAX_BODY = 4 * 1024 * 1024


class Blocked(RuntimeError):
    pass


def require(condition, reason):
    if not condition:
        raise Blocked(reason)


def read_json(path):
    path = Path(path)
    require(path.is_file() and path.stat().st_size <= MAX_BODY, "missing or oversized input file")
    return json.loads(path.read_text())


def save_json(path, value):
    body = (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode()
    with open(path, "xb") as stream:
        os.chmod(path, 0o600)
        stream.write(body)


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def target_url(value):
    parsed = urllib.parse.urlsplit(value)
    require(parsed.scheme == "http" and parsed.hostname in {"127.0.0.1", "localhost", "::1"},
            "the owned browser fixture must use an explicit loopback HTTP target")
    require(parsed.port is not None and not parsed.username and not parsed.password
            and parsed.path in {"", "/"} and not parsed.query and not parsed.fragment,
            "target must be a base URL with an explicit fixture port")
    require(parsed.port not in {48990, 8080, 18089, 15499, 15432}, "reserved non-fixture port")
    return value.rstrip("/")


def http(session, path, method="GET", body=None, authenticated=False):
    require(path.startswith("/") and not path.startswith("//"), "relative fixture route required")
    data = None if body is None else json.dumps(body, separators=(",", ":")).encode()
    headers = {"Accept": "application/json", "Gateway-Version": "2026-09-18"}
    if data is not None:
        headers["Content-Type"] = "application/json"
    if authenticated:
        key = os.environ.get("ARC_PAY_CALLER_KEY", "")
        require(re.fullmatch(r"pgk_[0-9a-f]{64}", key), "ARC_PAY_CALLER_KEY is missing or malformed")
        headers["Authorization"] = "Bearer " + key
    request = urllib.request.Request(target_url(session["target"]) + path,
                                     data=data, headers=headers, method=method)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    try:
        response = opener.open(request, timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    except (urllib.error.URLError, TimeoutError, OSError) as error:
        raise Blocked("fixture transport unavailable; outcome must not be retried automatically") from error
    with response:
        raw = response.read(MAX_BODY + 1)
        require(len(raw) <= MAX_BODY, "fixture response exceeds capture budget")
        try:
            value = json.loads(raw)
        except (ValueError, UnicodeDecodeError) as error:
            raise Blocked("fixture did not return a JSON observation") from error
        return response.status, value


def identity(session, bind=False):
    code, value = http(session, "/__qa/identity")
    require(code == 200, "fixture identity unavailable")
    require(value.get("environment") == "isolated-browser" and value.get("storage") == "postgres"
            and value.get("provider") == "controlled", "real owned PostgreSQL fixture required; memory mode is refused")
    require(value.get("run_id") == session["run_id"] and value.get("revision") == REVISION,
            "fixture run/revision differs from the pinned source")
    require(re.fullmatch(r"[0-9a-f]{64}", value.get("binary_sha256", "")), "fixture has no binary identity")
    require(re.fullmatch(r"qa_[A-Za-z0-9_]+", value.get("database_name", "")), "owned case database identity required")
    if not bind:
        for key in ("binary_sha256", "database_name", "pg_fingerprint"):
            require(value.get(key) == session["identity"].get(key), "fixture identity changed after binding")
    return value


def isolated_config(repo):
    path = Path(repo) / "tests/fixtures/.isolated-postgres.local.json"
    value = read_json(path)
    require(value.get("environment_kind") == "isolated_test", "isolated_test PostgreSQL configuration required")
    name = value.get("database_name", "")
    require(name and ("test" in name.lower() or "isolated" in name.lower()), "database name must identify an isolated test target")
    parsed = urllib.parse.urlsplit(value.get("dsn", ""))
    require(parsed.scheme in {"postgres", "postgresql"} and parsed.hostname,
            "isolated configuration must contain a PostgreSQL URL DSN")
    require(parsed.hostname not in {"localhost", "127.0.0.1", "::1"}, "pay_gateway requires a remote isolated PostgreSQL target")
    require(urllib.parse.unquote(parsed.path.lstrip("/")) == name, "DSN database differs from configured isolated target")
    require(value.get("purpose") and value.get("run_id"), "isolated PostgreSQL purpose/run identity missing")
    return value


def sql_json(session, query):
    psql = session.get("psql_path") or shutil.which("psql")
    require(psql, "psql is required for independent read-only PostgreSQL observations")
    config = isolated_config(session["pay_repo"])
    parsed = urllib.parse.urlsplit(config["dsn"])
    database = session["identity"]["database_name"]
    require(re.fullmatch(r"qa_[A-Za-z0-9_]+", database), "unowned case database")
    case_dsn = urllib.parse.urlunsplit(parsed._replace(path="/" + database))
    env = {"PATH": os.environ.get("PATH", ""), "LANG": "C.UTF-8", "PGDATABASE": case_dsn,
           "PGCONNECT_TIMEOUT": "5", "PGAPPNAME": "arc-independent-readback"}
    sql = "BEGIN READ ONLY; SET LOCAL statement_timeout='5000ms';\n" + query + ";\nROLLBACK;\n"
    try:
        result = subprocess.run([psql, "-X", "-A", "-t", "-q", "-w", "-v", "ON_ERROR_STOP=1"],
                                input=sql, capture_output=True, text=True, env=env, timeout=15)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise Blocked("independent PostgreSQL observation unavailable") from error
    require(result.returncode == 0, "independent PostgreSQL query failed; connection details are not printed")
    try:
        return json.loads(result.stdout)
    except ValueError as error:
        raise Blocked("PostgreSQL observation is not one complete JSON value") from error


def literal(value):
    require(isinstance(value, str) and "\x00" not in value, "invalid SQL subject identity")
    return "'" + value.replace("'", "''") + "'"


def database_snapshot(session):
    caller, payment, refund = map(literal, (session["caller_id"], session["payment_no"], session["refund_no"]))
    # The pinned host creates a new owned database and a new RatioProviders for
    # each reset (browser_host.rs:998-1003,1409-1413). Count every creation in
    # that database: filtering by an index under test would hide orphan sends.
    return sql_json(session, f"""
WITH selected AS (
  SELECT id, caller_id, kind, parent_order_id, merchant_order_no, channel_account_id,
         provider, provider_request_id, amount, currency
  FROM p_orders WHERE caller_id={caller}
    AND ((kind='payment' AND merchant_order_no={payment}) OR (kind='refund' AND merchant_order_no={refund}))
)
SELECT json_build_object(
 'database',current_database(),'role',current_user,
 'database_oid',(SELECT oid::bigint FROM pg_database WHERE datname=current_database()),
 'caller_enabled',EXISTS(SELECT 1 FROM p_callers WHERE id={caller} AND status='enabled'),
 'account_enabled',EXISTS(SELECT 1 FROM p_channel_accounts WHERE id='{ACCOUNT}' AND status='enabled'),
 'orders',COALESCE((SELECT json_agg(s ORDER BY kind,id) FROM selected s),'[]'::json),
 'payment_dispatches',(SELECT COUNT(*) FROM p_provider_dispatches WHERE operation='create_payment'),
 'refund_dispatches',(SELECT COUNT(*) FROM p_provider_dispatches WHERE operation='submit_refund'),
 'payment_confirmed',EXISTS(SELECT 1 FROM p_notify_jobs WHERE payload_status='succeeded'
    AND order_id IN (SELECT id FROM selected WHERE kind='payment')) OR
    EXISTS(SELECT 1 FROM p_callback_receipts WHERE result='verified' AND verified_status='succeeded'
    AND order_id IN (SELECT id FROM selected WHERE kind='payment')),
 'refund_notifications',(SELECT COUNT(*) FROM p_notify_jobs WHERE order_id IN (SELECT id FROM selected WHERE kind='refund')),
 'refund_callbacks',(SELECT COUNT(*) FROM p_callback_receipts WHERE order_id IN (SELECT id FROM selected WHERE kind='refund'))
)
""")


def normalize(session, database, observation, catalog):
    require(database.get("database") == session["identity"]["database_name"], "SQL observed another database")
    expected_db = session.get("database_binding")
    if expected_db:
        require(all(database.get(k) == expected_db[k] for k in ("database", "role", "database_oid")),
                "PostgreSQL identity changed after binding")
    require(observation.get("database", {}).get("name") == database["database"]
            and observation["database"].get("oid") == database["database_oid"],
            "fixture observer and independent SQL disagree on the database")
    provider_observation = observation.get("provider_observation", {})
    require(isinstance(provider_observation.get("errors"), list) and not provider_observation["errors"],
            "institution observer has missing or failed receipts")
    rows = database["orders"]
    payments = [row for row in rows if row["kind"] == "payment"]
    refunds = [row for row in rows if row["kind"] == "refund"]
    payment = payments[0] if len(payments) == 1 else None
    refund = refunds[0] if len(refunds) == 1 else None
    for row in rows:
        require(row["caller_id"] == session["caller_id"], "SQL observation escaped its bound caller filter")
    enabled_accounts = catalog.get("channel_accounts", [])
    available = any(row.get("id") == ACCOUNT and row.get("provider") == "huiju"
                    and "refund" in row.get("supported_operations", []) for row in enabled_accounts)
    parent_valid = bool(payment and refund and refund["parent_order_id"] == payment["id"])
    same_caller = bool(payment and refund and payment["caller_id"] == refund["caller_id"])
    same_account = bool(payment and refund and payment["channel_account_id"] == refund["channel_account_id"])
    peer = observation.get("provider_receipts")
    require(isinstance(peer, list), "independent institution receipts are missing")
    # The host observer returns the current case's newly constructed provider,
    # not a process-wide history (browser_host.rs:1514-1520). Keep total counts
    # independent of p_orders existence and of the requested provider. Check
    # their association separately so unrelated receipts cannot prove delivery.
    def received(kind):
        return sum(row.get("operation") == kind for row in peer)
    unexpected_receipts = 0
    for row in peer:
        operation = row.get("operation")
        if operation not in {"CreatePayment", "CreateRefund"}:
            continue
        original = payment if operation == "CreatePayment" else refund
        if (row.get("provider") != "huiju" or original is None
                or row.get("request_id") != original["provider_request_id"]):
            unexpected_receipts += 1
    values = {
        "actor.enabled": database["caller_enabled"] and available,
        "account.enabled": database["account_enabled"] and available,
        "payment.exists": bool(payments), "refund.exists": bool(refunds),
        "payment.rows": len(payments), "refund.rows": len(refunds),
        "payment.id": payment["id"] if payment else "",
        "refund.id": refund["id"] if refund else "",
        "provider.payment_dispatches": database["payment_dispatches"],
        "provider.refund_dispatches": database["refund_dispatches"],
        "provider.payment_received": received("CreatePayment"),
        "provider.refund_received": received("CreateRefund"),
        "provider.unexpected_receipts": unexpected_receipts,
        "provider.payment_confirmed": database["payment_confirmed"],
        "relation.parent_valid": parent_valid, "relation.same_caller": same_caller,
        "relation.same_account": same_account,
        "refund.notifications": database["refund_notifications"],
        "refund.callbacks": database["refund_callbacks"],
        "environment.controlled": True,
    }
    for alias, selected in (("payment", payment), ("refund", refund)):
        for attribute in ("amount", "currency", "channel_account_id", "provider"):
            values[alias + "." + attribute] = selected[attribute] if selected else None
    return {
        "state": "REFUND_INDEXED" if refunds else "PAYMENT_INDEXED" if payments else "EMPTY",
        "principal": session["caller_id"], "role": "CALLER", "scope": session["scope"],
        "login": values["actor.enabled"], "account_available": values["account.enabled"],
        "fixture_scope": values["actor.enabled"] and values["account.enabled"],
        "payment_present": bool(payments), "refund_present": bool(refunds),
        "values": values, "payment_id": values["payment.id"],
        "refund_id": values["refund.id"],
        "database": {key: database[key] for key in ("database", "role", "database_oid")},
        "evidence": {"sql_sha256": digest(database), "fixture_observer_sha256": digest(observation),
                     "catalog_sha256": digest(catalog), "orders": rows},
    }


def observe(session):
    identity(session)
    code, observation = http(session, "/__qa/observe")
    require(code == 200, "fixture independent observer is unavailable")
    code, catalog = http(session, "/api/v1/channel_accounts", authenticated=True)
    require(code == 200, "bound caller/account authentication could not be independently verified")
    return normalize(session, database_snapshot(session), observation, catalog)


def confirm_payment(session):
    before = observe(session)
    require(before["payment_id"] and not before["refund_present"], "payment preparation requires exactly its original payment")
    require(before["values"]["provider.payment_received"] == 1, "original payment has not reached the controlled institution exactly once")
    requests = []
    event = {"run_id": session["run_id"], "order_id": before["payment_id"]}
    code, value = http(session, "/__qa/provider_paid", "POST", event)
    requests.append({"entry": "/__qa/provider_paid", "input": event, "status": code, "actual": value})
    require(code == 200 and value.get("peer_paid") is True, "controlled external event was not accepted")
    query = {"channel_account_id": ACCOUNT, "merchant_order_no": session["payment_no"]}
    code, value = http(session, "/api/v1/payments/query", "POST", query, authenticated=True)
    requests.append({"entry": "/api/v1/payments/query", "input": query, "status": code, "actual": value})
    require(code == 200 and value.get("order_id") == before["payment_id"] and value.get("status") == "succeeded",
            "normal query did not confirm the controlled institution event; no refund should be sent")
    # The preparation engine reads SQL again; this response alone never establishes the goal.
    return {"requests": requests, "payment_id": before["payment_id"], "event_kind": "controlled-institution-paid"}


def preflight(repo, target, run_id):
    require(re.fullmatch(r"[A-Za-z0-9._-]{1,120}", run_id), "explicit QA_RUN_ID required")
    repo = str(Path(repo).resolve())
    require((Path(repo) / "tests/fixtures/browser_host.rs").is_file(), "pay_gateway checkout/browser fixture is missing")
    result = subprocess.run(["git", "-C", repo, "rev-parse", "HEAD"], capture_output=True, text=True)
    require(result.returncode == 0 and result.stdout.strip() == REVISION, "pay_gateway checkout differs from the reviewed revision")
    result = subprocess.run(["git", "-C", repo, "status", "--porcelain", "--untracked-files=no"], capture_output=True, text=True)
    require(result.returncode == 0 and not result.stdout.strip(), "pay_gateway tracked source changed; re-review before execution")
    config = isolated_config(repo)
    require(shutil.which("psql"), "psql is required for independent PostgreSQL readback")
    session = {"pay_repo": repo, "target": target_url(target), "run_id": run_id,
               "psql_path": str(Path(shutil.which("psql")).resolve())}
    session["identity"] = identity(session, bind=True)
    require(session["identity"].get("pg_fingerprint") == hashlib.sha256(config["dsn"].encode()).hexdigest(),
            "fixture and local configuration identify different PostgreSQL targets")
    binary = Path(repo) / "target/debug/examples/browser_fixture"
    require(binary.is_file(), "reviewed fixture binary is missing from the candidate checkout")
    with binary.open("rb") as stream:
        binary_hash = hashlib.file_digest(stream, "sha256").hexdigest()
    require(binary_hash == session["identity"]["binary_sha256"], "running fixture binary differs from the candidate build")
    return session


def bootstrap(args):
    session = preflight(args.pay_repo, args.target, args.run_id)
    out = Path(args.out).resolve()
    require(not out.exists(), "output directory already exists; a new run must keep old evidence")
    case_id = "arc_" + uuid.uuid4().hex[:16]
    code, seed = http(session, "/__qa/reset", "POST", {"run_id": args.run_id, "case_id": case_id})
    require(code == 200 and isinstance(seed.get("caller"), dict), "owned fixture reset did not return its seed")
    # The fixture itself owns and creates the new database; Arc never writes SQL to prepare state.
    session["identity"] = identity(session, bind=True)
    session.update({"case_id": case_id, "scope": "pay-owned-" + case_id,
                    "caller_id": seed["caller"]["id"], "payment_no": case_id + "_payment",
                    "refund_no": case_id + "_refund", "notify_url": seed["notify_url"],
                    "client_ip": seed["client_ip"]})
    key = seed["caller"]["key"]
    require(re.fullmatch(r"pgk_[0-9a-f]{64}", key), "fixture seed has an invalid caller key")
    require(seed.get("database_name") == session["identity"]["database_name"], "reset seed and observed database differ")
    os.environ["ARC_PAY_CALLER_KEY"] = key
    initial = observe(session)
    require(initial["state"] == "EMPTY" and initial["login"] and initial["account_available"],
            "new fixture was not independently observed in the required empty/active state")
    session["database_binding"] = initial["database"]
    out.mkdir(parents=True, mode=0o700)
    save_json(out / "caller-private.json", {"value": key})
    save_json(out / "session.json", session)
    save_json(out / "initial-observation.json", initial)
    print(json.dumps({"status": "PREPARED_ENVIRONMENT", "session": str(out / "session.json"),
                      "product_e2e": "NOT_RUN", "database": session["identity"]["database_name"]}))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    for name in ("preflight", "bootstrap"):
        command = sub.add_parser(name)
        command.add_argument("--pay-repo", required=True)
        command.add_argument("--target", required=True)
        command.add_argument("--run-id", required=True)
        if name == "bootstrap":
            command.add_argument("--out", required=True)
    for name in ("observe", "confirm-payment"):
        command = sub.add_parser(name)
        command.add_argument("--session", required=True)
    args = parser.parse_args()
    try:
        if args.command == "bootstrap":
            bootstrap(args)
        elif args.command == "preflight":
            session = preflight(args.pay_repo, args.target, args.run_id)
            print(json.dumps({"status": "READY_FOR_OWNED_FIXTURE_SETUP", "product_e2e": "NOT_RUN",
                              "revision": REVISION, "database": session["identity"]["database_name"]}))
        else:
            session = read_json(args.session)
            expected = os.environ.get("ARC_VERIFY_PRINCIPAL")
            require(not expected or expected == session["caller_id"], "adapter principal differs from Arc's frozen account")
            target = os.environ.get("ARC_VERIFY_TARGET")
            require(not target or target.rstrip("/") == session["target"], "adapter target differs from Arc's frozen environment")
            value = observe(session) if args.command == "observe" else confirm_payment(session)
            print(json.dumps(value, ensure_ascii=False))
    except (Blocked, OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        # Do not print library exception text that might contain a DSN or credential.
        reason = str(error) if isinstance(error, Blocked) else "adapter input/runtime failure; inspect protected local configuration"
        print(json.dumps({"status": "BLOCKED", "reason": reason, "product_e2e": "NOT_PROVEN"}))
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
