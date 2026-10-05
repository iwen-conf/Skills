#!/usr/bin/env python3
from __future__ import annotations

import argparse, json, operator, os, threading, time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, require_test_database_name, sha256_file
from sqlverify_config import load_config

OPS={"==":operator.eq,"!=":operator.ne,">=":operator.ge,">":operator.gt,"<=":operator.le,"<":operator.lt}

def _psycopg():
    try:
        import psycopg  # type: ignore
        return psycopg
    except ImportError as e: raise RuntimeError('psycopg 3 is required') from e

def run_assertions(conn, assertions:list[dict[str,Any]]) -> list[dict[str,Any]]:
    out=[]
    with conn.cursor() as cur:
        for a in assertions:
            cur.execute(a['sql'],a.get('params')); row=cur.fetchone(); actual=row[0] if row else None; op=a.get('op','=='); expected=a.get('expected'); passed=OPS[op](actual,expected)
            out.append({'id':a['id'],'actual':actual,'op':op,'expected':expected,'passed':passed})
    return out

def main()->int:
    ap=argparse.ArgumentParser(description='Rehearse PostgreSQL schema/data migration including lock probe, assertions and rollback')
    ap.add_argument('--config',required=True); ap.add_argument('--migration',required=True); ap.add_argument('--root',default='.'); ap.add_argument('--out',required=True)
    ns=ap.parse_args(); cfg=load_config(Path(ns.config)); root=Path(ns.root).resolve(); m=next((x for x in cfg.get('migration_rehearsal',[]) if x.get('id')==ns.migration),None)
    if not m: raise SystemExit(f'migration_rehearsal not found: {ns.migration}')
    started=time.monotonic(); ev=evidence_base('migration_rehearsal',f'migration.{ns.migration}'); dsn_env=cfg['database']['dsn_env']; dsn=os.environ.get(dsn_env)
    up=(root/m['up_sql_file']).resolve(); down=(root/m['down_sql_file']).resolve() if m.get('down_sql_file') else None
    ev['reproduction']={'dsn_env':dsn_env,'migration':ns.migration,'up_sql_sha256':sha256_file(up),'down_sql_sha256':sha256_file(down) if down else None,'transactional':m.get('transactional',True)}
    if not dsn: ev['violations'].append(f'missing env {dsn_env}'); json_dump(Path(ns.out),finalize_evidence(ev,started,'BLOCKED')); return 2
    try:
        psycopg=_psycopg()
        with psycopg.connect(dsn,autocommit=True) as c:
            with c.cursor() as cur: cur.execute('select current_database()'); db=cur.fetchone()[0]
        require_test_database_name(db,cfg['database']['safe_database_pattern'],bool(cfg['database'].get('allow_destructive',False)))
    except Exception as e:
        ev['violations'].append(repr(e)); json_dump(Path(ns.out),finalize_evidence(ev,started,'BLOCKED')); return 2

    up_sql=up.read_text(encoding='utf-8'); down_sql=down.read_text(encoding='utf-8') if down else None; observed={'database':db}
    try:
        with psycopg.connect(dsn,autocommit=True) as conn:
            observed['pre_assertions']=run_assertions(conn,m.get('pre_assertion',[]))
        for a in observed['pre_assertions']:
            if not a['passed']: ev['violations'].append(f"pre assertion {a['id']} failed")

        # Lock-risk probe: keep a blocker transaction open, try migration under small lock_timeout, always roll back probe.
        if m.get('lock_probe_sql'):
            ready=threading.Event(); release=threading.Event(); blocker_error=[]
            def blocker():
                try:
                    with psycopg.connect(dsn,autocommit=False) as bc:
                        with bc.cursor() as cur:
                            cur.execute(m['lock_probe_sql']); ready.set(); release.wait(float(m.get('lock_probe_hold_s',5)))
                        bc.rollback()
                except Exception as e: blocker_error.append(repr(e)); ready.set()
            t=threading.Thread(target=blocker,daemon=True); t.start(); ready.wait(5)
            lock_state=None; lock_err=None; probe_elapsed=None
            if blocker_error: ev['violations'].append(f'lock blocker failed: {blocker_error[0]}')
            else:
                st=time.monotonic()
                try:
                    with psycopg.connect(dsn,autocommit=False) as pc:
                        with pc.cursor() as cur:
                            cur.execute("select set_config('lock_timeout', %s, true)",(f"{int(m.get('lock_probe_timeout_ms',250))}ms",))
                            cur.execute(up_sql)
                        pc.rollback()
                except Exception as e:
                    lock_state=getattr(e,'sqlstate',None); lock_err=repr(e)
                probe_elapsed=round((time.monotonic()-st)*1000,3)
            release.set(); t.join(2)
            observed['lock_probe']={'sqlstate':lock_state,'error':lock_err,'elapsed_ms':probe_elapsed}
            if m.get('require_nonblocking',False) and lock_state=='55P03': ev['violations'].append('migration lock probe hit lock_timeout (55P03); declared require_nonblocking=true')
            if m.get('expect_lock_timeout',False) and lock_state!='55P03': ev['violations'].append(f"expected lock_timeout 55P03 but got {lock_state}")

        # Apply migration.
        st=time.monotonic(); transactional=bool(m.get('transactional',True))
        with psycopg.connect(dsn,autocommit=not transactional) as conn:
            with conn.cursor() as cur:
                cur.execute("select set_config('statement_timeout', %s, false)",(f"{int(m.get('statement_timeout_ms',60000))}ms",))
                cur.execute("select set_config('lock_timeout', %s, false)",(f"{int(m.get('apply_lock_timeout_ms',5000))}ms",))
                cur.execute(up_sql)
            if transactional: conn.commit()
        observed['apply_elapsed_ms']=round((time.monotonic()-st)*1000,3)
        if m.get('max_apply_ms') is not None and observed['apply_elapsed_ms']>float(m['max_apply_ms']): ev['violations'].append(f"migration apply {observed['apply_elapsed_ms']}ms > {m['max_apply_ms']}ms")

        with psycopg.connect(dsn,autocommit=True) as conn: observed['post_assertions']=run_assertions(conn,m.get('post_assertion',[]))
        for a in observed['post_assertions']:
            if not a['passed']: ev['violations'].append(f"post assertion {a['id']} failed")

        if down_sql and m.get('verify_rollback',True):
            st=time.monotonic()
            with psycopg.connect(dsn,autocommit=not transactional) as conn:
                with conn.cursor() as cur: cur.execute(down_sql)
                if transactional: conn.commit()
            observed['rollback_elapsed_ms']=round((time.monotonic()-st)*1000,3)
            with psycopg.connect(dsn,autocommit=True) as conn: observed['rollback_assertions']=run_assertions(conn,m.get('rollback_assertion',[]))
            for a in observed['rollback_assertions']:
                if not a['passed']: ev['violations'].append(f"rollback assertion {a['id']} failed")
        elif m.get('rollback_required',False): ev['violations'].append('rollback_required=true but down_sql_file missing')
        ev['observed']=observed; status='PASS' if not ev['violations'] else 'FAIL'
    except Exception as e:
        ev['observed']=observed; ev['violations'].append(repr(e)); status='FAIL'
    json_dump(Path(ns.out),finalize_evidence(ev,started,status)); print(json.dumps(ev,ensure_ascii=False,indent=2,default=str)); return 0 if status=='PASS' else 2

if __name__=='__main__': raise SystemExit(main())
