#!/usr/bin/env python3
from __future__ import annotations

import argparse, json, os, time
from pathlib import Path
from typing import Any

from pgbench_runner import run_one, safe_db_check
from sqlverify_common import evidence_base, finalize_evidence, json_dump, percentile, redact_dsn, which
from sqlverify_config import load_config


def main() -> int:
    ap=argparse.ArgumentParser(description='Run ramp/impulse/overload/recovery pgbench phases and verify graceful recovery')
    ap.add_argument('--config',required=True); ap.add_argument('--profile',required=True); ap.add_argument('--root',default='.'); ap.add_argument('--evidence-dir',required=True)
    ns=ap.parse_args(); root=Path(ns.root).resolve(); cfg=load_config(Path(ns.config)); profile=next((x for x in cfg.get('overload_profile',[]) if x.get('id')==ns.profile),None)
    if not profile: raise SystemExit(f'overload_profile not found: {ns.profile}')
    base=next((w for w in cfg.get('workload',[]) if w.get('id')==profile.get('workload')),None)
    if not base: raise SystemExit(f"workload not found: {profile.get('workload')}")
    outdir=Path(ns.evidence_dir); outdir.mkdir(parents=True,exist_ok=True); ev_id=f"overload.{ns.profile}"; started=time.monotonic(); ev=evidence_base('overload_recovery',ev_id)
    dsn_env=cfg['database']['dsn_env']; dsn=os.environ.get(dsn_env)
    ev['reproduction']={'dsn_env':dsn_env,'profile':ns.profile,'phases':profile.get('phase',[]),'workload':profile.get('workload')}
    if dsn: ev['reproduction']['dsn_redacted']=redact_dsn(dsn)
    if not dsn or not which('pgbench') or not which('psql'):
        ev['violations'].append('DATABASE_URL/pgbench/psql unavailable'); json_dump(outdir/f'{ev_id}.json',finalize_evidence(ev,started,'BLOCKED')); return 2
    safe,db=safe_db_check(dsn,cfg['database']['safe_database_pattern'])
    if not safe and not (cfg['database'].get('allow_destructive',False) and os.environ.get('SQLVERIFY_ALLOW_DESTRUCTIVE')=='1'):
        ev['violations'].append(f'unsafe database target {db!r}'); json_dump(outdir/f'{ev_id}.json',finalize_evidence(ev,started,'BLOCKED')); return 2

    phases=[]; rawdir=outdir/'raw'/ev_id; rawdir.mkdir(parents=True,exist_ok=True)
    try:
        for i,p in enumerate(profile.get('phase',[]),1):
            w=dict(base); w['id']=f"{base['id']}-{p.get('name',i)}"; w['clients']=[int(p.get('clients',profile.get('clients',max(base.get('clients',[1])))))]
            w['duration_s']=int(p.get('duration_s',profile.get('duration_s',30))); w['warmup_s']=0; w['repetitions']=1
            if p.get('rate_tps') is None: w.pop('rate_tps',None)
            else: w['rate_tps']=float(p['rate_tps'])
            if p.get('latency_limit_ms') is not None: w['latency_limit_ms']=float(p['latency_limit_ms'])
            r=run_one(root,w,w['clients'][0],i,dsn,rawdir)
            parsed=r['parsed']; successes=int(r.get('processed') or 0); failures=int(r.get('failed') or 0); den=successes+failures
            phase={
              'name':p.get('name',str(i)),'kind':p.get('kind','load'),'rate_tps':p.get('rate_tps'),'clients':w['clients'][0],
              'tps':r.get('tps'),'p95_ms':parsed.get('latency_ms',{}).get('p95'),'p99_ms':parsed.get('latency_ms',{}).get('p99'),
              'schedule_lag_p99_ms':(parsed.get('schedule_lag_ms') or {}).get('p99'),'failure_rate':(failures/den if den else 0.0),
              'failed':failures,'processed':successes,'retries':r.get('total_retries',0),'returncode':r['returncode'],'seed':r['random_seed']
            }
            phases.append(phase)
            max_fail=float(p.get('max_failure_rate',profile.get('max_failure_rate',1.0)))
            if phase['failure_rate']>max_fail: ev['violations'].append(f"phase {phase['name']} failure_rate={phase['failure_rate']:.6f} > {max_fail}")
            if p.get('max_p99_ms') is not None and (phase['p99_ms'] is None or phase['p99_ms']>float(p['max_p99_ms'])):
                ev['violations'].append(f"phase {phase['name']} p99_ms={phase['p99_ms']} > {p['max_p99_ms']}")
            if r['returncode']!=0: ev['violations'].append(f"phase {phase['name']} pgbench exit={r['returncode']}")
        # Recovery gate compares a named recovery phase with named nominal phase.
        nominal_name=profile.get('nominal_phase'); recovery_name=profile.get('recovery_phase')
        nominal=next((p for p in phases if p['name']==nominal_name),None); recovery=next((p for p in phases if p['name']==recovery_name),None)
        recovery_eval=None
        if nominal_name and recovery_name:
            if not nominal or not recovery:
                ev['violations'].append('nominal/recovery phase missing')
            else:
                p99_ratio=(recovery['p99_ms']/nominal['p99_ms']) if recovery['p99_ms'] and nominal['p99_ms'] else None
                tps_ratio=(recovery['tps']/nominal['tps']) if recovery['tps'] and nominal['tps'] else None
                recovery_eval={'p99_ratio':p99_ratio,'tps_ratio':tps_ratio}
                if p99_ratio is None or p99_ratio>float(profile.get('max_recovery_p99_ratio',1.25)):
                    ev['violations'].append(f'recovery p99 ratio {p99_ratio} exceeds max {profile.get("max_recovery_p99_ratio",1.25)}')
                if tps_ratio is None or tps_ratio<float(profile.get('min_recovery_tps_ratio',0.90)):
                    ev['violations'].append(f'recovery TPS ratio {tps_ratio} below min {profile.get("min_recovery_tps_ratio",0.90)}')
        ev['observed']={'database':db,'phases':phases,'recovery':recovery_eval}
        status='PASS' if not ev['violations'] else 'FAIL'
    except Exception as e:
        ev['violations'].append(repr(e)); status='BLOCKED'
    json_dump(outdir/f'{ev_id}.json',finalize_evidence(ev,started,status)); print(json.dumps(ev,ensure_ascii=False,indent=2,default=str)); return 0 if status=='PASS' else 2

if __name__=='__main__': raise SystemExit(main())
