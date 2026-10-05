#!/usr/bin/env python3
"""Validate R8 keep-radius evidence and report constraint scores; never fill missing metrics."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys

TABLES = ('kept_class_info_table', 'kept_field_info_table', 'kept_method_info_table')
COUNTS = ('live_class_count', 'live_field_count', 'live_method_count')
FLAGS = {'DONT_OPTIMIZE': '-dontoptimize', 'DONT_OBFUSCATE': '-dontobfuscate', 'DONT_SHRINK': '-dontshrink'}


def nonnegative(value):
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        raise ValueError('Expected nonnegative integer count/id')
    return value


def id_map(rows):
    result = {}
    for row in rows:
        identifier = nonnegative(row['id'])
        if identifier in result:
            raise ValueError('Duplicate id')
        result[identifier] = row
    return result


def analyze(data):
    required = (*TABLES, 'keep_constraints_table', 'keep_rule_blast_radius_table', 'global_keep_rule_blast_radius_table')
    if not isinstance(data, dict) or any(not isinstance(data.get(key), list) for key in required):
        raise ValueError('Missing or invalid evidence tables')
    counts = [nonnegative(data['build_info'][key]) for key in COUNTS]
    denominator = sum(counts)
    if denominator == 0:
        raise ValueError('No live items: quantitative analysis unavailable')
    constraints = id_map(data['keep_constraints_table'])
    rules = id_map(data['keep_rule_blast_radius_table'])
    for row in constraints.values():
        if not isinstance(row['constraints'], list) or any(flag not in FLAGS for flag in row['constraints']):
            raise ValueError('Unknown constraint/schema')
    for rule in rules.values():
        if nonnegative(rule['constraints_id']) not in constraints:
            raise ValueError('Dangling constraint reference')
    totals = dict.fromkeys(FLAGS, 0)
    for table, live_count in zip(TABLES, counts):
        rows = id_map(data[table])
        if len(rows) > live_count:
            raise ValueError('Kept items exceed live count')
        for row in rows.values():
            if not isinstance(row['kept_by'], list) or any(nonnegative(identifier) not in rules for identifier in row['kept_by']):
                raise ValueError('Dangling keep-rule reference')
            present = {flag for identifier in row['kept_by'] for flag in constraints[rules[identifier]['constraints_id']]['constraints']}
            for flag in present:
                totals[flag] += 1
    globals_sources = [row['source'].strip().lower().split() for row in id_map(data['global_keep_rule_blast_radius_table']).values()]
    scores = {flag: 0.0 if any(source and token == source[0] for source in globals_sources) else 100.0 * (1 - totals[flag] / denominator) for flag, token in FLAGS.items()}
    impacts = []
    for identifier, rule in rules.items():
        radius = rule['blast_radius']
        sizes = []
        for key, limit in zip(('class_blast_radius', 'field_blast_radius', 'method_blast_radius'), counts):
            values = radius[key]
            if not isinstance(values, list):
                raise ValueError('Invalid blast-radius list')
            size = len({nonnegative(value) for value in values})
            if size > limit:
                raise ValueError('Blast radius exceeds live count')
            sizes.append(size)
        if not isinstance(radius['subsumed_by'], list) or any(nonnegative(parent) not in rules for parent in radius['subsumed_by']):
            raise ValueError('Dangling subsumption reference')
        impacts.append({'id': identifier, 'source': rule['source'], 'impact': sum(sizes), 'impact_pct': 100 * sum(sizes) / denominator, 'classes': sizes[0], 'fields': sizes[1], 'methods': sizes[2], 'subsumed_by': radius['subsumed_by']})
    return {'mode': 'quantitative', 'live_items': denominator, 'scores': scores, 'rules': sorted(impacts, key=lambda row: (-row['impact'], row['id']))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument('--json', type=Path)
    source.add_argument('--proto', type=Path)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.json:
            data = json.loads(args.json.read_text())
        else:
            from google.protobuf import json_format
            import keep_radius_pb2
            bundle = keep_radius_pb2.BlastRadiusContainer()
            bundle.ParseFromString(args.proto.read_bytes())
            encoded = bundle.SerializeToString(deterministic=True)
            bundle.DiscardUnknownFields()
            if encoded != bundle.SerializeToString(deterministic=True):
                raise ValueError('Unknown protobuf fields: binding/version mismatch')
            data = json_format.MessageToDict(bundle, always_print_fields_with_no_presence=True, preserving_proto_field_name=True)
        result = analyze(data)
        # Refuse reuse of a prior output; failed runs cannot masquerade as fresh evidence.
        with args.out.open('x', encoding='utf-8') as output:
            output.write(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    except Exception as error:
        print(f'R8 analysis failed: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
