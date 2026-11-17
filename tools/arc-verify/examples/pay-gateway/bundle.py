#!/usr/bin/env python3
"""Create an Arc runner/config bundle for an already observed owned pay fixture."""
from __future__ import annotations

import argparse
import copy
import json
import os
from pathlib import Path
import sys

import adapter


GRAPH = "pay-gateway-index-fixture-v1"


def eq(subject, attribute, kind, value):
    return {"op": "eq", "left": {"ref": {"subject": subject, "attribute": attribute}},
            "right": {"literal": {"type": kind, "value": value}}}


def build(session_path, scenario="core"):
    session_path = Path(session_path).resolve()
    session = adapter.read_json(session_path)
    here = Path(__file__).resolve().parent
    model = adapter.read_json(here / "model.json")
    adapter.require(model["id"] == GRAPH, "bundle model identity changed")
    scope = session["scope"]
    environment = adapter.ENVIRONMENT
    actor_bindings = {"actor": "actor.primary.principal", "payment": "entity.payment.id", "refund": "entity.refund.id"}
    observed_attributes = {
        "actor.enabled", "account.enabled", "payment.exists", "refund.exists", "payment.rows", "refund.rows",
        "payment.id", "refund.id", "provider.payment_dispatches", "provider.refund_dispatches",
        "provider.payment_received", "provider.refund_received", "provider.payment_confirmed",
        "provider.unexpected_receipts",
        "relation.parent_valid", "relation.same_caller", "relation.same_account", "refund.notifications",
        "refund.callbacks", "environment.controlled",
        "payment.amount", "payment.currency", "payment.channel_account_id", "payment.provider",
        "refund.amount", "refund.currency", "refund.channel_account_id", "refund.provider",
    }
    declared = {subject["id"] + "." + attribute["name"] for subject in model["business_contract"]["subjects"]
                for attribute in subject["attributes"]}
    def command(name):
        return {"kind": "command", "args": [sys.executable, str(here / "adapter.py"), name, "--session", str(session_path)],
                "directory": session["pay_repo"], "env_refs": {"ARC_PAY_CALLER_KEY": "env://ARC_PAY_CALLER_KEY"},
                "success_exit_codes": [0]}
    oracle = {"id": "pay-store", "boundary": "independent read-only PostgreSQL + owned fixture institution receipts",
              "call": command("observe"), "state_path": "/state", "principal_path": "/principal",
              "role_path": "/role", "scope_path": "/scope", "conditions": {"C_FIXTURE_SCOPE": "/fixture_scope"},
              "measurements": {name: "/values/" + name for name in sorted(observed_attributes & declared)}}
    def extraction(alias, required=True):
        return {"name": alias + "_id", "source": "oracle", "oracle_id": "pay-store", "path": "/" + alias + "_id",
                "entity_kind": alias, "entity_alias": alias, "owner_actor": "primary", "required": required}
    payment_body = {"channel_account_id": adapter.ACCOUNT, "merchant_order_no": session["payment_no"],
                    "amount": 100, "currency": "CNY", "payment_method": "alipay", "subject": "Arc isolated reference chain",
                    "notify_url": session["notify_url"], "client_ip": session["client_ip"]}
    refund_body = {"channel_account_id": adapter.ACCOUNT, "merchant_order_no": session["payment_no"],
                   "merchant_refund_no": session["refund_no"], "amount": 100, "currency": "CNY",
                   "notify_url": session["notify_url"], "reason": "Arc isolated reference chain"}
    def operation(node, path, body, transition="", branch="", response=None, extract=None):
        call = {"kind": "http", "method": "POST", "path": path, "body": copy.deepcopy(body),
                "headers": {"Gateway-Version": "2026-09-18"}, "credential_header": "Authorization", "credential_prefix": "Bearer ",
                "success_statuses": [200], "rejection_statuses": [401, 404, 409, 422]}
        if extract:
            # Target ID/cardinality failures belong to the frozen comparison.
            # Preparation still requires a unique entity before any next action.
            call["extract"] = [extraction(extract, required=not branch)]
        step = {"node": node, "actor": "primary", "call": call, "oracles": [copy.deepcopy(oracle)],
                "subject_bindings": {key: value for key, value in actor_bindings.items()
                                     if key in {subject["id"] for subject in model["business_contract"]["subjects"]}},
                "input_measurements": {"request.amount": "/amount", "request.currency": "/currency",
                                       "request.channel_account_id": "/channel_account_id"}}
        if transition:
            step["transition"] = transition
        if branch:
            step["business_branch"] = branch
        if response:
            step["response_measurements"] = response
        return step
    create = operation("CREATE_PAYMENT", "/api/v1/payments", payment_body, "TR_CREATE_PAYMENT_INDEX", "BR_CREATE_PAYMENT_INDEX",
                       {"reply.order_id": "/order_id"}, "payment")
    submit = operation("SUBMIT_REFUND", "/api/v1/refunds", refund_body, "TR_CREATE_REFUND_INDEX", "BR_CREATE_REFUND_INDEX",
                       {"reply.order_id": "/order_id", "reply.status": "/status"}, "refund")
    duplicate = operation("SUBMIT_REFUND", "/api/v1/refunds", refund_body, "TR_DUPLICATE_REFUND", "BR_DUPLICATE_REFUND",
                          {"reply.code": "/error/code", "reply.resource_id": "/error/resource_id"})
    missing = operation("SUBMIT_REFUND", "/api/v1/refunds", refund_body, "TR_MISSING_PARENT", "BR_MISSING_PARENT",
                        {"reply.code": "/error/code"})
    prepare_create = operation("PREP_CREATE_PAYMENT_ACTION", "/api/v1/payments", payment_body,
                               response={"reply.order_id": "/order_id"}, extract="payment")
    prepare_create.pop("input_measurements")
    confirm = {"node": "PREP_CONFIRM_PAYMENT_ACTION", "actor": "primary", "call": command("confirm-payment"),
               "oracles": [copy.deepcopy(oracle)], "subject_bindings": {"actor": "actor.primary.principal"}}
    # Capture both real HTTP operations and their actual responses in the setup RAW.
    confirm["response_measurements"] = {"preparation.requests": "/requests"}
    preparation = {"goal": {"op": "and", "args": [eq("payment", "exists", "boolean", True),
                                                        eq("provider", "payment_confirmed", "boolean", True)]},
                   "max_steps": 4, "max_states": 32,
                   "observer": {"node": "SUBMIT_REFUND", "actor": "primary", "oracles": [copy.deepcopy(oracle)],
                                "subject_bindings": {"actor": "actor.primary.principal"}},
                   "adapters": {"PREP_CREATE_PAYMENT": prepare_create, "PREP_CONFIRM_PAYMENT": confirm}}
    scenarios = {
        "core": ("TEST_REFUND_DUPLICATE_CHAIN", [submit, duplicate], "REFUND_INDEXED", True),
        "refund": ("TEST_REFUND_WITH_AUTOPREPARATION", [submit], "REFUND_INDEXED", True),
        "create": ("TEST_CREATE_PAYMENT_INDEX", [create], "PAYMENT_INDEXED", False),
        "missing-parent": ("TEST_MISSING_PARENT", [missing], "EMPTY", False),
        "duplicate": ("TEST_DUPLICATE_REFUND", [duplicate], "REFUND_INDEXED", False),
    }
    adapter.require(scenario in scenarios, "unknown bounded scenario")
    test_id, steps, checkpoint, prepare = scenarios[scenario]
    checks = [{"left": "payment.rows", "operator": "eq", "value": 0 if checkpoint == "EMPTY" else 1},
              {"left": "refund.rows", "operator": "eq", "value": 1 if checkpoint == "REFUND_INDEXED" else 0}]
    if checkpoint == "REFUND_INDEXED":
        checks += [{"left": "provider.refund_dispatches", "operator": "eq", "value": 1},
                   {"left": "provider.refund_received", "operator": "eq", "value": 1},
                   {"left": "relation.parent_valid", "operator": "eq", "value": True},
                   {"left": "relation.same_caller", "operator": "eq", "value": True},
                   {"left": "relation.same_account", "operator": "eq", "value": True}]
    inputs = [str(here / "adapter.py"), str(session_path)]
    if session.get("psql_path"):
        inputs.append(session["psql_path"])
    runner = {"schema_version": 1, "id": "pay-" + session["case_id"] + "-" + scenario, "graph": GRAPH, "test": test_id,
              "isolation_scope": scope, "timeout_ms": 120000, "step_timeout_ms": 25000, "parallelism": 1,
              "input_files": inputs, "steps": steps,
              "recovery": {"mode": "checkpoint", "isolation_scope": scope, "steps": [], "state": checkpoint,
                           "oracle": copy.deepcopy(oracle), "conditions": {"C_FIXTURE_SCOPE": True}, "checks": checks}}
    if prepare:
        runner["preparation"] = preparation
    if scenario == "duplicate":
        readback = command("observe")
        readback["extract"] = [{**extraction(alias), "source": "response_body"} for alias in ("payment", "refund")]
        for item in readback["extract"]:
            item.pop("oracle_id")
        runner["setup"] = [{"actor": "primary", "call": readback}]
    fixtures = {"fixtures": [
        {"id": "caller-" + session["case_id"], "kind": "account", "state": "ACTIVE",
         "secretRef": "env://ARC_PAY_CALLER_KEY", "refs": {"role": "CALLER", "principal": session["caller_id"], "environment": environment}},
        {"id": "instance-" + session["case_id"], "kind": "business", "state": "EMPTY",
         "refs": {"environment": environment, "scope": scope}},
    ]}
    return runner, fixtures, {"environments": {environment: {"approved": True, "url": session["target"]}}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--session", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--scenario", default="core", choices=["core", "refund", "create", "missing-parent", "duplicate"])
    args = parser.parse_args()
    try:
        runner, fixtures, config = build(args.session, args.scenario)
        out = Path(args.out).resolve()
        adapter.require(not out.exists(), "bundle already exists; preserve the original runner and evidence")
        out.mkdir(parents=True, mode=0o700)
        for name, value in (("runner.json", runner), ("fixtures.json", fixtures), ("config.jsonc", config)):
            adapter.save_json(out / name, value)
        print(json.dumps({"status": "BUNDLE_CREATED", "product_e2e": "NOT_RUN", "path": str(out), "runner": runner["id"],
                          "task": GRAPH + "/" + runner["test"], "account": fixtures["fixtures"][0]["id"],
                          "fixture": fixtures["fixtures"][1]["id"]}))
    except (adapter.Blocked, OSError, ValueError, KeyError) as error:
        print(json.dumps({"status": "BLOCKED", "reason": str(error) if isinstance(error, adapter.Blocked) else "invalid bundle inputs"}))
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
