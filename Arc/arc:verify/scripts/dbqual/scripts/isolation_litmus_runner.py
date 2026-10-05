#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import re
import threading
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, require_test_database_name
from sqlverify_config import load_config

_SAFE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")
_ALLOWED_ISO = {"read committed", "repeatable read", "serializable"}


def _psycopg():
    try:
        import psycopg  # type: ignore
        return psycopg
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def _sqlstate(exc: BaseException) -> str | None:
    return getattr(exc, "sqlstate", None) or getattr(getattr(exc, "diag", None), "sqlstate", None)


def _worker(psycopg, dsn: str, isolation: str, case_type: str, actor: int, barrier: threading.Barrier, out: dict[str, Any], timeout_ms: int) -> None:
    row: dict[str, Any] = {"actor": actor, "sqlstate": None, "committed": False, "read": None}
    try:
        with psycopg.connect(dsn, autocommit=False) as conn:
            with conn.cursor() as cur:
                cur.execute(f"begin isolation level {isolation}")
                cur.execute("select set_config('statement_timeout', %s, true)", (f"{timeout_ms}ms",))
                if case_type == "write_skew":
                    cur.execute("select count(*) from sqlverify_litmus.doctors where on_call")
                    n = int(cur.fetchone()[0]); row["read"] = n
                    barrier.wait(timeout=max(1, timeout_ms/1000))
                    if n >= 2:
                        cur.execute("update sqlverify_litmus.doctors set on_call=false where id=%s", (actor,))
                elif case_type == "lost_update_detection":
                    cur.execute("select v from sqlverify_litmus.counter where id=1")
                    v = int(cur.fetchone()[0]); row["read"] = v
                    barrier.wait(timeout=max(1, timeout_ms/1000))
                    cur.execute("update sqlverify_litmus.counter set v=%s where id=1", (v+1,))
                else:
                    raise ValueError(f"unsupported litmus type: {case_type}")
                conn.commit(); row["committed"] = True
    except BaseException as e:
        row["sqlstate"] = _sqlstate(e); row["error"] = repr(e)
        try:
            barrier.abort()
        except Exception:
            pass
    out[str(actor)] = row


def main() -> int:
    ap=argparse.ArgumentParser(description="Run standard PostgreSQL isolation litmus tests against a disposable schema")
    ap.add_argument("--config", required=True); ap.add_argument("--case", required=True); ap.add_argument("--out", required=True)
    ns=ap.parse_args(); cfg=load_config(Path(ns.config))
    case=next((x for x in cfg.get("isolation_litmus",[]) if x.get("id")==ns.case),None)
    if not case: raise SystemExit(f"isolation_litmus not found: {ns.case}")
    start=time.monotonic(); ev=evidence_base("isolation_litmus",f"isolation.{ns.case}")
    dsn_env=cfg["database"]["dsn_env"]; dsn=os.environ.get(dsn_env)
    if not dsn:
        ev["violations"].append(f"missing env {dsn_env}"); json_dump(Path(ns.out),finalize_evidence(ev,start,"BLOCKED")); return 2
    isolation=str(case.get("isolation","serializable")).lower()
    case_type=str(case.get("type","write_skew"))
    if isolation not in _ALLOWED_ISO:
        ev["violations"].append(f"unsupported isolation {isolation}"); json_dump(Path(ns.out),finalize_evidence(ev,start,"FAIL")); return 2
    timeout_ms=int(case.get("statement_timeout_ms",5000)); obs: dict[str,Any]={"type":case_type,"isolation":isolation}
    try:
        psycopg=_psycopg()
        with psycopg.connect(dsn,autocommit=True) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database()"); db=cur.fetchone()[0]
                require_test_database_name(db,cfg["database"]["safe_database_pattern"],bool(cfg["database"].get("allow_destructive",False)))
                cur.execute("create schema if not exists sqlverify_litmus")
                if case_type=="write_skew":
                    cur.execute("create table if not exists sqlverify_litmus.doctors(id int primary key, on_call boolean not null)")
                    cur.execute("truncate sqlverify_litmus.doctors")
                    cur.execute("insert into sqlverify_litmus.doctors values (1,true),(2,true)")
                elif case_type=="lost_update_detection":
                    cur.execute("create table if not exists sqlverify_litmus.counter(id int primary key, v int not null)")
                    cur.execute("truncate sqlverify_litmus.counter")
                    cur.execute("insert into sqlverify_litmus.counter values (1,0)")
                else: raise ValueError(f"unsupported type {case_type}")
        barrier=threading.Barrier(2); results: dict[str,Any]={}
        th=[threading.Thread(target=_worker,args=(psycopg,dsn,isolation,case_type,i,barrier,results,timeout_ms),daemon=True) for i in (1,2)]
        for t in th:t.start()
        for t in th:t.join(timeout=max(2,float(case.get("timeout_s",10))))
        if any(t.is_alive() for t in th): raise RuntimeError("litmus worker timeout")
        with psycopg.connect(dsn,autocommit=True) as conn:
            with conn.cursor() as cur:
                if case_type=="write_skew":
                    cur.execute("select count(*) from sqlverify_litmus.doctors where on_call"); final=int(cur.fetchone()[0]); obs["final_on_call_count"]=final
                    if case.get("min_final_on_call") is not None and final < int(case["min_final_on_call"]): ev["violations"].append(f"write-skew invariant violated: final_on_call={final} < {case['min_final_on_call']}")
                else:
                    cur.execute("select v from sqlverify_litmus.counter where id=1"); final=int(cur.fetchone()[0]); obs["final_value"]=final
                    if case.get("expected_final_value") is not None and final != int(case["expected_final_value"]): ev["violations"].append(f"final counter {final} != {case['expected_final_value']}")
        states=[r.get("sqlstate") for r in results.values() if r.get("sqlstate")]
        counts={s:states.count(s) for s in sorted(set(states))}; obs["sessions"]=results; obs["sqlstate_counts"]=counts; obs["commits"]=sum(1 for r in results.values() if r.get("committed"))
        allowed=set(case.get("allowed_sqlstates",[])); required=set(case.get("required_sqlstates",[]))
        unexpected=[s for s in states if s not in allowed]
        if unexpected: ev["violations"].append(f"unexpected SQLSTATEs: {unexpected}")
        missing=[s for s in required if counts.get(s,0)==0]
        if missing: ev["violations"].append(f"required SQLSTATEs not observed: {missing}")
        if case.get("max_serialization_failures") is not None and counts.get("40001",0)>int(case["max_serialization_failures"]): ev["violations"].append("serialization failure budget exceeded")
        status="PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status="BLOCKED"
    ev["observed"]=obs; ev["reproduction"]={"case":case,"dsn_env":dsn_env}
    json_dump(Path(ns.out),finalize_evidence(ev,start,status)); print(json.dumps(ev,ensure_ascii=False,indent=2,default=str))
    return 0 if status=="PASS" else 2

if __name__=="__main__": raise SystemExit(main())
