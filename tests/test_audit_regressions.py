"""Executable counterexamples from the 2026-10-05 skill audit.

These verify local scripts/examples, not remote services or agent compliance.
"""
from __future__ import annotations

from copy import deepcopy
import importlib.util
import itertools
import json
import os
from pathlib import Path
import random
import re
import shutil
import subprocess
import sys

import pytest

ROOT = Path(__file__).resolve().parents[1]


def load(relative):
    spec = importlib.util.spec_from_file_location(Path(relative).stem, ROOT / relative)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def command(argv, **kwargs):
    return subprocess.run(argv, text=True, capture_output=True, **kwargs)


def r8_evidence():
    # Six live items; two prevented from optimization, one from shrinking.
    return {
        'build_info': {'live_class_count': 3, 'live_field_count': 1, 'live_method_count': 2},
        'keep_constraints_table': [
            {'id': 0, 'constraints': ['DONT_OPTIMIZE']},
            {'id': 1, 'constraints': ['DONT_SHRINK']},
        ],
        'keep_rule_blast_radius_table': [
            {'id': 0, 'source': '-keep,allowshrinking class A', 'constraints_id': 0,
             'blast_radius': {'class_blast_radius': [0], 'field_blast_radius': [],
                              'method_blast_radius': [0], 'subsumed_by': []}},
            {'id': 1, 'source': '-keep,allowoptimization class B', 'constraints_id': 1,
             'blast_radius': {'class_blast_radius': [1], 'field_blast_radius': [],
                              'method_blast_radius': [], 'subsumed_by': []}},
        ],
        'global_keep_rule_blast_radius_table': [],
        'kept_class_info_table': [{'id': 0, 'kept_by': [0]}, {'id': 1, 'kept_by': [1]}],
        'kept_field_info_table': [],
        'kept_method_info_table': [{'id': 0, 'kept_by': [0]}],
    }


def test_r8_scores_match_independent_counts():
    result = load('Android/r8-analyzer/scripts/analyze.py').analyze(r8_evidence())
    assert result['scores'] == pytest.approx({
        'DONT_OPTIMIZE': 100 * 4 / 6,
        'DONT_OBFUSCATE': 100,
        'DONT_SHRINK': 100 * 5 / 6,
    })
    assert [(r['id'], r['impact']) for r in result['rules']] == [(0, 2), (1, 1)]


@pytest.mark.parametrize('fault', ['empty', 'zero', 'missing', 'dangling', 'duplicate', 'enum', 'bounds'])
def test_r8_rejects_missing_or_invalid_evidence(fault, tmp_path):
    data = deepcopy(r8_evidence())
    if fault == 'empty': data = {}
    if fault == 'zero': data['build_info'] = dict.fromkeys(data['build_info'], 0)
    if fault == 'missing': del data['build_info']['live_field_count']
    if fault == 'dangling': data['kept_class_info_table'][0]['kept_by'] = [999]
    if fault == 'duplicate': data['keep_constraints_table'].append(data['keep_constraints_table'][0])
    if fault == 'enum': data['keep_constraints_table'][0]['constraints'] = ['FUTURE_CONSTRAINT']
    if fault == 'bounds': data['build_info']['live_class_count'] = 1
    source, output = tmp_path / 'evidence.json', tmp_path / 'analysis_result.txt'
    source.write_text(json.dumps(data))
    result = command([sys.executable, str(ROOT / 'Android/r8-analyzer/scripts/analyze.py'),
                      '--json', str(source), '--out', str(output)])
    assert result.returncode != 0
    assert not output.exists()
    assert 'R8 analysis failed' in result.stderr


def test_r8_does_not_overwrite_prior_result(tmp_path):
    source, output = tmp_path / 'in.json', tmp_path / 'analysis_result.txt'
    source.write_text(json.dumps(r8_evidence()))
    output.write_text('previous evidence')
    result = command([sys.executable, str(ROOT / 'Android/r8-analyzer/scripts/analyze.py'),
                      '--json', str(source), '--out', str(output)])
    assert result.returncode != 0
    assert output.read_text() == 'previous evidence'


def test_r8_protobuf_roundtrip(tmp_path):
    python = os.environ.get('R8_PROTO_PYTHON', sys.executable)
    if command([python, '-c', 'import google.protobuf']).returncode:
        pytest.skip('Set R8_PROTO_PYTHON to an interpreter with protobuf >= 6.33.4')
    source = tmp_path / 'evidence.json'
    source.write_text(json.dumps(r8_evidence()))
    code = '''import sys, json
sys.path.insert(0, sys.argv[1])
from google.protobuf import json_format
import keep_radius_pb2
bundle = json_format.ParseDict(json.load(open(sys.argv[2])), keep_radius_pb2.BlastRadiusContainer())
open(sys.argv[3], 'wb').write(bundle.SerializeToString())
'''
    proto, output = tmp_path / 'input.pb', tmp_path / 'analysis_result.txt'
    result = command([python, '-c', code, str(ROOT / 'Android/r8-analyzer/scripts'), str(source), str(proto)])
    assert result.returncode == 0, result.stderr
    result = command([python, str(ROOT / 'Android/r8-analyzer/scripts/analyze.py'),
                      '--proto', str(proto), '--out', str(output)])
    assert result.returncode == 0, result.stderr
    assert json.loads(output.read_text()) == load('Android/r8-analyzer/scripts/analyze.py').analyze(r8_evidence())
    # Unknown wire field 127 must not silently turn incompatible data into scores.
    proto.write_bytes(proto.read_bytes() + bytes([0xf8, 0x07, 0x01]))
    unknown_output = tmp_path / 'unknown.txt'
    result = command([python, str(ROOT / 'Android/r8-analyzer/scripts/analyze.py'),
                      '--proto', str(proto), '--out', str(unknown_output)])
    assert result.returncode != 0
    assert 'Unknown protobuf fields' in result.stderr
    assert not unknown_output.exists()


def event(identifier, start, end, busy=True, declined=False):
    return dict(id=identifier, start=start, end=end, busy=busy, declined=declined)


def test_nested_events_touching_and_free_window():
    summarize = load('Lark/lark-workflow-standup-report/scripts/intervals.py').summarize
    events = [event('A', 9*60, 12*60), event('B', 10*60, 10*60+30),
              event('C', 11*60, 11*60+30), event('touch', 12*60, 13*60),
              event('free', 8*60, 14*60, busy=False), event('declined', 8*60, 14*60, declined=True)]
    result = summarize(events, (8*60, 14*60))
    assert set(result['conflicts']) == {('A', 'B'), ('A', 'C')}
    assert result['free'] == [[480, 540], [780, 840]]
    assert summarize(events)['free'] is None
    assert summarize(events, (600, 630))['free'] == []


def test_interval_sweep_matches_pairwise_oracle():
    summarize = load('Lark/lark-workflow-standup-report/scripts/intervals.py').summarize
    rng = random.Random(51)
    for _ in range(100):
        events = [event(str(i), (start := rng.randrange(50)), start+rng.randrange(1, 20),
                        rng.choice([True, False]), rng.choice([True, False])) for i in range(15)]
        expected = {frozenset((a['id'], b['id'])) for a, b in itertools.combinations(events, 2)
                    if a['busy'] and b['busy'] and not a['declined'] and not b['declined']
                    and max(a['start'], b['start']) < min(a['end'], b['end'])}
        result = summarize(events, (10, 40))
        assert {frozenset(pair) for pair in result['conflicts']} == expected
        expected_free = {t for t in range(10, 40) if not any(
            e['busy'] and not e['declined'] and e['start'] <= t < e['end'] for e in events)}
        assert {t for lo, hi in result['free'] for t in range(lo, hi)} == expected_free


def test_interval_rejects_unknown_state_and_duplicate_ids():
    summarize = load('Lark/lark-workflow-standup-report/scripts/intervals.py').summarize
    with pytest.raises(ValueError): summarize([event('a', 1, 2, busy=None)])
    with pytest.raises(ValueError): summarize([event('a', 1, 2), event('a', 2, 3)])


def converter(text, **env_overrides):
    env = {**os.environ, 'CNB_WEB_ENDPOINT': 'https://cnb.cool',
           'CNB_REPO_SLUG': 'group/repo', 'CNB_BRANCH': 'release/2', **env_overrides}
    return command(['node', str(ROOT / 'CNB/cnb/scripts/convertLink.js'), text], env=env)


def test_converter_supported_links_code_and_bare_paths():
    text = '[doc](./docs/a.md) <img src="../a.png"> bare/path.md\n`[code](./stay.md)`\n```md\n[x](./keep.md)\n```'
    result = converter(text)
    assert result.returncode == 0
    assert result.stdout.strip() == ('[doc](https://cnb.cool/group/repo/-/git/raw/release/2/docs/a.md) '
        '<img src="https://cnb.cool/group/repo/-/git/raw/release/2/a.png"> bare/path.md\n'
        '`[code](./stay.md)`\n```md\n[x](./keep.md)\n```')
    assert converter('[x](https://example.org/a) [issue](/-/issues/1)').stdout.strip() == (
        '[x](https://example.org/a) [issue](https://cnb.cool/group/repo/-/issues/1)')


@pytest.mark.parametrize('overrides', [{'CNB_REPO_SLUG': ''}, {'CNB_BRANCH': ''},
                                       {'CNB_WEB_ENDPOINT': 'file:///tmp'}, {'CNB_WEB_ENDPOINT': 'https://u:p@host'}])
def test_converter_missing_environment_fails(overrides):
    result = converter('[x](./a.md)', **overrides)
    assert result.returncode == 2
    assert result.stdout == ''


def test_nondefault_cnb_base_and_unrelated_changes(tmp_path):
    origin, work = tmp_path / 'origin', tmp_path / 'work'
    def git(*args, cwd=work):
        result = command(['git', *args], cwd=cwd)
        assert result.returncode == 0, result.stderr
        return result.stdout.strip()
    git('init', '-b', 'main', str(origin), cwd=tmp_path)
    git('config', 'user.email', 'audit@example.invalid', cwd=origin)
    git('config', 'user.name', 'Audit', cwd=origin)
    (origin / 'tracked').write_text('main')
    git('add', 'tracked', cwd=origin); git('commit', '-m', 'main', cwd=origin)
    git('switch', '-c', 'release/2', cwd=origin)
    (origin / 'tracked').write_text('release')
    git('commit', '-am', 'release', cwd=origin)
    base = git('rev-parse', 'HEAD', cwd=origin)
    git('clone', str(origin), str(work), cwd=tmp_path)
    (work / 'unrelated').write_text('user work')
    git('add', 'unrelated')
    initial_staged = git('diff', '--cached')
    text = (ROOT / 'Waza/check/references/delivery.md').read_text()
    snippet = next(block for block in re.findall(r'```bash\n(.*?)```', text, re.S) if 'git check-ref-format' in block)
    snippet = '\n'.join(line.strip() for line in snippet.splitlines())
    result = command(['bash', '-e', '-c', snippet], cwd=work,
                     env={**os.environ, 'base_ref': 'release/2', 'work_branch': 'auto/test-1'})
    assert result.returncode == 0, result.stderr
    assert git('rev-parse', 'HEAD') == base
    assert git('branch', '--show-current') == 'auto/test-1'
    assert (work / 'unrelated').read_text() == 'user work'
    assert git('diff', '--cached') == initial_staged


def test_trace_helper_emits_entire_json_line(tmp_path):
    if not shutil.which('go'): pytest.skip('Go toolchain unavailable')
    text = (ROOT / 'Arc/arc:code/references/perf-probes.md').read_text()
    helper = re.findall(r'```go\n(.*?)```', text, re.S)[0]
    (tmp_path / 'trace.go').write_text(helper)
    (tmp_path / 'go.mod').write_text('module example.invalid/audit\n\ngo 1.21\n')
    (tmp_path / 'trace_test.go').write_text(r'''package tracing
import ("bytes"; "encoding/json"; "log/slog"; "strings"; "testing")
func TestJSON(t *testing.T) {
 var out bytes.Buffer
 logger := slog.New(slog.NewJSONHandler(&out, nil))
 request, stage := "id\"\nnext", "fetch\n\"quoted"
 NewTrace(logger, request, stage).Done()
 if strings.Count(out.String(), "\n") != 1 { t.Fatal(out.String()) }
 var value map[string]any
 if err := json.Unmarshal(out.Bytes(), &value); err != nil { t.Fatal(err) }
 if value["request_id"] != request || value["stage"] != stage || value["event"] != "ssr_trace" { t.Fatal(value) }
 if value["cost_ms"].(float64) < 0 { t.Fatal(value) }
}
''')
    result = command(['go', 'test', './...'], cwd=tmp_path, env={**os.environ, 'GOWORK': 'off'})
    assert result.returncode == 0, result.stdout + result.stderr


@pytest.mark.parametrize('failure', [None, 'ohpm', 'hvigorw'])
def test_arkts_wrapper_uses_project_and_preserves_failures(tmp_path, failure):
    project, bins, outside = tmp_path / 'project with spaces', tmp_path / 'bin', tmp_path / 'elsewhere'
    for p in (project, bins, outside): p.mkdir()
    log = tmp_path / 'calls'
    for path, name in [(bins / 'ohpm', 'ohpm'), (project / 'hvigorw', 'hvigorw')]:
        path.write_text('#!/bin/sh\nprintf "%s|%s|%s\\n" "'+name+'" "$PWD" "$*" >> "$AUDIT_CALLS"\n'
                        + ('exit 7\n' if failure == name else 'exit 0\n'))
        path.chmod(0o755)
    registry = project / '.ohpmrc'
    registry.write_text('registry=https://registry.example.invalid')
    result = command(['bash', str(ROOT / 'HarmonyOS/harmonyos/scripts/arkts/run.sh'), str(project)],
                     cwd=outside, env={**os.environ, 'PATH': str(bins)+os.pathsep+os.environ['PATH'], 'AUDIT_CALLS': str(log)})
    assert result.returncode == (7 if failure else 0)
    calls = log.read_text().splitlines()
    assert calls[0] == f'ohpm|{project}|install --all'
    assert len(calls) == (1 if failure == 'ohpm' else 2)
    if len(calls) == 2: assert calls[1] == f'hvigorw|{project}|assembleApp'
    assert registry.read_text() == 'registry=https://registry.example.invalid'


def test_source_distribution_is_validated_but_not_installed(tmp_path, monkeypatch):
    from arc_core.usecase import validate_skills
    from arc_core.usecase.validate_skills import run_validation
    monkeypatch.setattr(validate_skills, 'SOURCE_SKILL_PATHS', ('reliability-engineering-skill/SKILL.md',))
    source = tmp_path / 'reliability-engineering-skill'
    source.mkdir()
    (source / 'SKILL.md').write_text('---\nname: reliability-engineering\ndescription: source\n---\n[broken](missing.md)\n')
    errors, _, count = run_validation(tmp_path)
    assert count == 1
    assert any('broken relative link' in e for e in errors)
    assert load('scripts/sync_skills.py').list_skill_dirs(tmp_path) == []
