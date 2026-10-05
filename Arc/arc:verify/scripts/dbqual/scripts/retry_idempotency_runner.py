#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import secrets
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, require_test_database_name, run
from sqlverify_config import load_config


def _psycopg():
    try:
        import psycopg  # type: ignore
        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def _cmd(v: Any) -> list[str]:
    if not isinstance(v, list) or not v or not all(isinstance(x,str) and x for x in v):
        raise ValueError("command must be a non-empty argv array")
    return list(v)


def _safe_command(cmd: list[str]) -> list[str]:
    # Commands are evidence, but obvious inline secrets are redacted.
    out=[]
    for x in cmd:
        low=x.lower()
        if any(k in low for k in ("password=","token=","secret=","authorization:")):
            if "=" in x: out.append(x.split("=",1)[0]+"=***")
            else: out.append("***")
        else: out.append(x)
    return out


def main() -> int:
    ap=argparse.ArgumentParser(description="Exercise end-to-end retry/idempotency semantics using a stable operation key and DB oracles")
    ap.add_argument("--config",required=True); ap.add_argument("--case",required=True); ap.add_argument("--root",default="."); ap.add_argument("--out",required=True)
    ap.add_argument("--ack-isolated-environment",action="store_true")
    ns=ap.parse_args(); cfg=load_config(Path(ns.config)); root=Path(ns.root).resolve()
    case=next((x for x in cfg.get("retry_idempotency",[]) if x.get("id")==ns.case),None)
    if not case: raise SystemExit(f"retry_idempotency not found: {ns.case}")
    start=time.monotonic(); ev=evidence_base("retry_idempotency",f"retry.{ns.case}")
    dsn_env=cfg["database"]["dsn_env"]; dsn=os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}"); json_dump(Path(ns.out),finalize_evidence(ev,start,"BLOCKED")); return 2
    if case.get("fault_command") and not ns.ack_isolated_environment:
        ev["violations"].append("fault_command requires --ack-isolated-environment"); json_dump(Path(ns.out),finalize_evidence(ev,start,"BLOCKED")); return 2
    obs: dict[str,Any]={"attempts":[]}
    key=str(case.get("idempotency_key") or f"sqlverify-{ns.case}-{secrets.token_hex(8)}")
    key_env=str(case.get("idempotency_key_env","SQLVERIFY_IDEMPOTENCY_KEY")); obs["idempotency_key_sha256"] = __import__('hashlib').sha256(key.encode()).hexdigest()
    try:
        psycopg=_psycopg()
        with psycopg.connect(dsn,autocommit=True) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database()"); db=cur.fetchone()[0]
                require_test_database_name(db,cfg["database"]["safe_database_pattern"],bool(cfg["database"].get("allow_destructive",False)))
        prepare=case.get("prepare_command")
        if prepare:
            cp=run(_cmd(prepare),cwd=root,timeout=float(case.get("command_timeout_s",60)))
            obs["prepare"]={"returncode":cp.returncode,"stdout_tail":cp.stdout[-2000:],"stderr_tail":cp.stderr[-2000:]}
            if cp.returncode!=0: raise RuntimeError(f"prepare_command failed rc={cp.returncode}")
        op=_cmd(case["operation_command"]); attempts=int(case.get("attempts",2)); allowed=set(int(x) for x in case.get("allowed_returncodes",[0]))
        env={key_env:key}
        for i in range(attempts):
            t0=time.monotonic(); cp=run(op,cwd=root,env=env,timeout=float(case.get("command_timeout_s",60)))
            rec={"attempt":i+1,"returncode":cp.returncode,"elapsed_ms":round((time.monotonic()-t0)*1000,3),"stdout_tail":cp.stdout[-2000:],"stderr_tail":cp.stderr[-2000:]}
            obs["attempts"].append(rec)
            if cp.returncode not in allowed: ev["violations"].append(f"operation attempt {i+1} rc={cp.returncode} not in {sorted(allowed)}")
            if i==0 and case.get("fault_command"):
                fp=run(_cmd(case["fault_command"]),cwd=root,timeout=float(case.get("fault_timeout_s",60)))
                obs["fault"]={"returncode":fp.returncode,"stdout_tail":fp.stdout[-2000:],"stderr_tail":fp.stderr[-2000:]}
                if fp.returncode!=0: ev["violations"].append(f"fault_command failed rc={fp.returncode}")
            if float(case.get("retry_delay_ms",0))>0 and i+1<attempts: time.sleep(float(case["retry_delay_ms"])/1000)
        assertions=[]
        with psycopg.connect(dsn,autocommit=False) as conn:
            with conn.cursor() as cur:
                cur.execute("set transaction read only")
                for a in case.get("assertion",[]):
                    cur.execute("select set_config('statement_timeout', %s, true)",(f"{int(a.get('statement_timeout_ms',10000))}ms",))
                    cur.execute(a["sql"]); row=cur.fetchone(); actual=row[0] if row else None
                    opx=a.get("op","=="); exp=a.get("expected"); ok={"==":actual==exp,"!=":actual!=exp,">":actual>exp,">=":actual>=exp,"<":actual<exp,"<=":actual<=exp}[opx]
                    assertions.append({"id":a["id"],"actual":actual,"op":opx,"expected":exp,"passed":ok})
                    if not ok: ev["violations"].append(f"idempotency assertion {a['id']} failed: {actual!r} {opx} {exp!r}")
                conn.rollback()
        obs["assertions"]=assertions
        status="PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status="BLOCKED"
    ev["observed"]=obs; ev["reproduction"]={"case_id":ns.case,"operation_command":_safe_command(_cmd(case.get("operation_command",["missing"]))),"idempotency_key_env":key_env,"dsn_env":dsn_env}
    json_dump(Path(ns.out),finalize_evidence(ev,start,status)); print(json.dumps(ev,ensure_ascii=False,indent=2,default=str))
    return 0 if status=="PASS" else 2

if __name__=="__main__": raise SystemExit(main())
