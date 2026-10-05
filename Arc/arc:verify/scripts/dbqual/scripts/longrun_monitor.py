#!/usr/bin/env python3
from __future__ import annotations

import argparse, json, os, subprocess, time
from pathlib import Path
from typing import Any

from sqlverify_common import evidence_base, finalize_evidence, json_dump, percentile, require_test_database_name, redact_dsn


def _psycopg():
    try:
        import psycopg  # type: ignore
        return psycopg
    except ImportError as e: raise RuntimeError('psycopg 3 is required') from e


def snap(conn) -> dict[str,Any]:
    out={}
    with conn.cursor() as cur:
        cur.execute("""select count(*), count(*) filter(where state='active'), count(*) filter(where state='idle in transaction'),
                    count(*) filter(where wait_event_type='Lock'),
                    coalesce(max(extract(epoch from (now()-xact_start))*1000) filter(where xact_start is not null),0)
                    from pg_stat_activity where datname=current_database()""")
        r=cur.fetchone(); out.update({'connections':r[0],'active':r[1],'idle_in_tx':r[2],'lock_waiters':r[3],'max_xact_age_ms':float(r[4] or 0)})
        cur.execute("select xact_commit,xact_rollback,temp_files,temp_bytes,deadlocks,blks_read,blks_hit from pg_stat_database where datname=current_database()")
        r=cur.fetchone(); out['db']={'xact_commit':r[0],'xact_rollback':r[1],'temp_files':r[2],'temp_bytes':r[3],'deadlocks':r[4],'blks_read':r[5],'blks_hit':r[6]}
        try:
            cur.execute("select wal_records,wal_fpi,wal_bytes,wal_buffers_full from pg_stat_wal"); r=cur.fetchone(); out['wal']={'wal_records':r[0],'wal_fpi':r[1],'wal_bytes':float(r[2]),'wal_buffers_full':r[3]}
        except Exception: conn.rollback(); out['wal']=None
    return out


def delta(a:dict[str,Any]|None,b:dict[str,Any]|None)->dict[str,Any]|None:
    if not a or not b:return None
    out={}
    for k in set(a)|set(b):
        try: out[k]=float(b.get(k,0))-float(a.get(k,0))
        except Exception: pass
    return out


def main()->int:
    ap=argparse.ArgumentParser(description='Long-running DB resource/lock observer; optionally wraps a workload command')
    ap.add_argument('--dsn-env',default='DATABASE_URL'); ap.add_argument('--safe-database-pattern',default='(_test|_bench|sqlverify)$'); ap.add_argument('--duration-s',type=int,required=True); ap.add_argument('--sample-s',type=float,default=5); ap.add_argument('--out',required=True); ap.add_argument('--max-idle-in-tx',type=int,default=0); ap.add_argument('--max-lock-waiters',type=int); ap.add_argument('--max-xact-age-ms',type=float); ap.add_argument('command',nargs=argparse.REMAINDER)
    ns=ap.parse_args(); started=time.monotonic(); ev=evidence_base('longrun_monitor','longrun.resource'); dsn=os.environ.get(ns.dsn_env); ev['reproduction']={'dsn_env':ns.dsn_env,'duration_s':ns.duration_s,'sample_s':ns.sample_s,'command':ns.command}
    if dsn:ev['reproduction']['dsn_redacted']=redact_dsn(dsn)
    if not dsn:ev['violations'].append(f'missing env {ns.dsn_env}'); json_dump(Path(ns.out),finalize_evidence(ev,started,'BLOCKED'));return 2
    try:
        psycopg=_psycopg(); conn=psycopg.connect(dsn,autocommit=True)
        with conn.cursor() as cur:cur.execute('select current_database()');db=cur.fetchone()[0]
        require_test_database_name(db,ns.safe_database_pattern,False)
    except Exception as e:ev['violations'].append(repr(e));json_dump(Path(ns.out),finalize_evidence(ev,started,'BLOCKED'));return 2
    proc=None;samples=[]
    try:
        if ns.command:
            cmd=ns.command[1:] if ns.command and ns.command[0]=='--' else ns.command
            proc=subprocess.Popen(cmd,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
        deadline=time.monotonic()+ns.duration_s
        while time.monotonic()<deadline:
            x=snap(conn);x['t_ms']=round((time.monotonic()-started)*1000,3);samples.append(x);time.sleep(min(ns.sample_s,max(0,deadline-time.monotonic())))
        if proc:
            try:out,err=proc.communicate(timeout=5)
            except subprocess.TimeoutExpired:proc.terminate();out,err=proc.communicate(timeout=5);ev['violations'].append('wrapped command exceeded monitor duration and was terminated')
            if proc.returncode!=0:ev['violations'].append(f'wrapped command exit={proc.returncode}: {err[-1000:]}')
            ev['observed']['command']={'returncode':proc.returncode,'stdout_tail':out[-3000:],'stderr_tail':err[-3000:]}
        idle_peak=max((x['idle_in_tx'] for x in samples),default=0); lock_peak=max((x['lock_waiters'] for x in samples),default=0); age_peak=max((x['max_xact_age_ms'] for x in samples),default=0); conn_peak=max((x['connections'] for x in samples),default=0)
        if idle_peak>ns.max_idle_in_tx:ev['violations'].append(f'idle-in-transaction peak {idle_peak} > {ns.max_idle_in_tx}')
        if ns.max_lock_waiters is not None and lock_peak>ns.max_lock_waiters:ev['violations'].append(f'lock waiters peak {lock_peak} > {ns.max_lock_waiters}')
        if ns.max_xact_age_ms is not None and age_peak>ns.max_xact_age_ms:ev['violations'].append(f'max transaction age {age_peak:.1f}ms > {ns.max_xact_age_ms}ms')
        ev['observed'].update({'database':db,'samples':samples[-5000:],'peaks':{'connections':conn_peak,'idle_in_tx':idle_peak,'lock_waiters':lock_peak,'max_xact_age_ms':age_peak},'db_delta':delta(samples[0].get('db') if samples else None,samples[-1].get('db') if samples else None),'wal_delta':delta(samples[0].get('wal') if samples else None,samples[-1].get('wal') if samples else None)})
        status='PASS' if not ev['violations'] else 'FAIL'
    except Exception as e:ev['violations'].append(repr(e));status='FAIL'
    finally:
        try:conn.close()
        except Exception:pass
    json_dump(Path(ns.out),finalize_evidence(ev,started,status));print(json.dumps(ev,ensure_ascii=False,indent=2,default=str));return 0 if status=='PASS' else 2
if __name__=='__main__':raise SystemExit(main())
