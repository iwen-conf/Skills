#!/usr/bin/env python3
from __future__ import annotations

import argparse, json, os, subprocess, time
from pathlib import Path
from typing import Any
from sqlverify_common import evidence_base, finalize_evidence, json_dump, json_load, sha256_file


def run_cmd(argv:list[str],timeout:float)->dict[str,Any]:
    st=time.monotonic()
    try:
        cp=subprocess.run(argv,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=timeout)
        return {'argv':argv,'returncode':cp.returncode,'elapsed_ms':round((time.monotonic()-st)*1000,3),'stdout_tail':cp.stdout[-3000:],'stderr_tail':cp.stderr[-3000:]}
    except Exception as e:return {'argv':argv,'returncode':None,'elapsed_ms':round((time.monotonic()-st)*1000,3),'error':repr(e)}

def main()->int:
    ap=argparse.ArgumentParser(description='Orchestrate isolated fault-injection scenario using argv commands; no shell')
    ap.add_argument('--scenario-file',required=True);ap.add_argument('--out',required=True);ap.add_argument('--ack-isolated-environment',action='store_true')
    ns=ap.parse_args();started=time.monotonic();ev=evidence_base('fault_scenario','fault.scenario');p=Path(ns.scenario_file);spec=json_load(p);ev['id']='fault.'+str(spec.get('id','scenario'));ev['reproduction']={'scenario_file':str(p),'sha256':sha256_file(p)}
    if not ns.ack_isolated_environment or not spec.get('isolated_environment_required',True):
        ev['violations'].append('fault injection requires explicit --ack-isolated-environment and isolated_environment_required=true');json_dump(Path(ns.out),finalize_evidence(ev,started,'BLOCKED'));return 2
    records=[];status='PASS'
    try:
        timeout=float(spec.get('command_timeout_s',60))
        for phase in ('prepare','before_oracle'):
            for cmd in spec.get(phase,[]):
                r=run_cmd(cmd,timeout);records.append({'phase':phase,**r});
                if r.get('returncode')!=0:raise RuntimeError(f'{phase} failed: {r}')
        target=spec.get('target')
        proc=None
        if target:
            proc=subprocess.Popen(target,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True);records.append({'phase':'target_start','argv':target,'pid':proc.pid})
        time.sleep(float(spec.get('fault_after_s',1)))
        for cmd in spec.get('inject',[]):
            r=run_cmd(cmd,timeout);records.append({'phase':'inject',**r});
            if r.get('returncode')!=0:raise RuntimeError(f'inject failed: {r}')
        time.sleep(float(spec.get('fault_hold_s',1)))
        for cmd in spec.get('recover',[]):
            r=run_cmd(cmd,timeout);records.append({'phase':'recover',**r});
            if r.get('returncode')!=0:raise RuntimeError(f'recover failed: {r}')
        if proc:
            try:out,err=proc.communicate(timeout=float(spec.get('target_timeout_s',60)))
            except subprocess.TimeoutExpired:proc.terminate();out,err=proc.communicate(timeout=5);raise RuntimeError('target timed out')
            records.append({'phase':'target_end','returncode':proc.returncode,'stdout_tail':out[-3000:],'stderr_tail':err[-3000:]})
            allowed=set(spec.get('allowed_target_returncodes',[0]));
            if proc.returncode not in allowed:ev['violations'].append(f'target returncode {proc.returncode} not in {sorted(allowed)}')
        for cmd in spec.get('oracle',[]):
            r=run_cmd(cmd,timeout);records.append({'phase':'oracle',**r});
            if r.get('returncode')!=0:ev['violations'].append(f'oracle failed: {r}')
        if ev['violations']:status='FAIL'
    except Exception as e:ev['violations'].append(repr(e));status='FAIL'
    finally:
        for cmd in spec.get('cleanup',[]):records.append({'phase':'cleanup',**run_cmd(cmd,float(spec.get('command_timeout_s',60)))})
    ev['observed']={'records':records};json_dump(Path(ns.out),finalize_evidence(ev,started,status));print(json.dumps(ev,ensure_ascii=False,indent=2));return 0 if status=='PASS' else 2
if __name__=='__main__':raise SystemExit(main())
