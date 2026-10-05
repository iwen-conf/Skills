#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, require_test_database_name, run
from sqlverify_config import load_config


def _psycopg():
    try:
        import psycopg  # type: ignore
        from psycopg.rows import dict_row  # type: ignore
        return psycopg, dict_row
    except ImportError as e:
        raise RuntimeError("psycopg 3 is required") from e


def _cmd(v: Any) -> list[str]:
    if not isinstance(v,list) or not v or not all(isinstance(x,str) and x for x in v): raise ValueError("command must be non-empty argv array")
    return list(v)


def main() -> int:
    ap=argparse.ArgumentParser(description="Audit PostgreSQL version/extension/collation compatibility and optional upgrade-check command")
    ap.add_argument("--config",required=True); ap.add_argument("--case",required=True); ap.add_argument("--root",default="."); ap.add_argument("--out",required=True)
    ns=ap.parse_args(); cfg=load_config(Path(ns.config)); root=Path(ns.root).resolve(); case=next((x for x in cfg.get("compatibility_audit",[]) if x.get("id")==ns.case),None)
    if not case: raise SystemExit(f"compatibility_audit not found: {ns.case}")
    start=time.monotonic(); ev=evidence_base("compatibility_audit",f"compatibility.{ns.case}")
    envname=case.get("dsn_env",cfg["database"]["dsn_env"]); dsn=os.environ.get(envname)
    if not dsn:
        ev["violations"].append(f"missing env {envname}"); json_dump(Path(ns.out),finalize_evidence(ev,start,"BLOCKED")); return 2
    obs: dict[str,Any]={}
    try:
        psycopg,dict_row=_psycopg()
        with psycopg.connect(dsn,autocommit=True,row_factory=dict_row) as conn:
            with conn.cursor() as cur:
                cur.execute("select current_database() as db,current_setting('server_version_num')::int as server_version_num,version() as version")
                meta=cur.fetchone(); require_test_database_name(meta["db"],case.get("safe_database_pattern",cfg["database"]["safe_database_pattern"]),bool(case.get("allow_destructive",False))); obs.update(meta)
                major=int(meta["server_version_num"])//10000; obs["major"]=major
                if case.get("expected_major") is not None and major!=int(case["expected_major"]): ev["violations"].append(f"PostgreSQL major {major} != expected {case['expected_major']}")
                cur.execute("select extname,extversion from pg_extension order by extname"); exts=cur.fetchall(); obs["extensions"]=exts
                expected=case.get("expected_extensions",{})
                by={r["extname"]:r["extversion"] for r in exts}
                for name,ver in expected.items():
                    if name not in by: ev["violations"].append(f"required extension missing: {name}")
                    elif ver not in (None,"*") and str(by[name])!=str(ver): ev["violations"].append(f"extension {name} version {by[name]} != {ver}")
                # Database collation version drift is a material upgrade risk when supported by the locale provider.
                try:
                    cur.execute("""
                        select datname, datcollversion, pg_database_collation_actual_version(oid) as actual_version
                        from pg_database where datallowconn order by datname
                    """)
                    cols=cur.fetchall(); mism=[r for r in cols if r.get("datcollversion") is not None and r.get("actual_version") is not None and r["datcollversion"]!=r["actual_version"]]
                except Exception:
                    cols=[]; mism=[]
                obs["database_collation_versions"]=cols; obs["collation_version_mismatches"]=mism
                if case.get("require_no_collation_version_mismatch",True) and mism: ev["violations"].append(f"database collation version mismatches: {len(mism)}")
        if case.get("check_command"):
            cp=run(_cmd(case["check_command"]),cwd=root,timeout=float(case.get("command_timeout_s",1800)))
            obs["check_command"]={"returncode":cp.returncode,"stdout_tail":cp.stdout[-8000:],"stderr_tail":cp.stderr[-8000:]}
            if cp.returncode!=0: ev["violations"].append(f"compatibility check command failed rc={cp.returncode}")
        for i,cmd in enumerate(case.get("smoke_command",[])):
            cp=run(_cmd(cmd),cwd=root,timeout=float(case.get("smoke_timeout_s",300)))
            obs.setdefault("smoke_commands",[]).append({"index":i,"returncode":cp.returncode,"stdout_tail":cp.stdout[-3000:],"stderr_tail":cp.stderr[-3000:]})
            if cp.returncode!=0: ev["violations"].append(f"smoke_command[{i}] failed rc={cp.returncode}")
        status="PASS" if not ev["violations"] else "FAIL"
    except Exception as e:
        ev["violations"].append(repr(e)); status="BLOCKED"
    ev["observed"]=obs; ev["reproduction"]={"case":case,"dsn_env":envname}
    json_dump(Path(ns.out),finalize_evidence(ev,start,status)); print(json.dumps(ev,ensure_ascii=False,indent=2,default=str))
    return 0 if status=="PASS" else 2

if __name__=="__main__": raise SystemExit(main())
