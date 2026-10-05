"""Pure interval analysis after the caller resolves API fields and timezone.

Each event needs a unique string id, integer start/end timestamps, and explicit
busy/declined booleans. Unknown API values must be resolved before calling.
"""


def summarize(events, window=None):
    ids = set()
    busy = []
    for event in events:
        identifier = event["id"]
        start, end = event["start"], event["end"]
        if not isinstance(identifier, str) or not identifier or identifier in ids:
            raise ValueError("Event IDs must be unique nonempty strings")
        ids.add(identifier)
        if type(start) is not int or type(end) is not int or start >= end:
            raise ValueError("Expected increasing integer timestamps")
        if type(event["busy"]) is not bool or type(event["declined"]) is not bool:
            raise ValueError("Busy/declined must be explicit booleans")
        if event["busy"] and not event["declined"]:
            busy.append((start, end, identifier))
    busy.sort()
    active, conflicts, merged = [], [], []
    for start, end, identifier in busy:
        active = [item for item in active if item[1] > start]
        conflicts.extend((item[2], identifier) for item in active)
        active.append((start, end, identifier))
        if merged and start <= merged[-1][1]:
            merged[-1][1] = max(merged[-1][1], end)
        else:
            merged.append([start, end])
    free = None
    if window is not None:
        lo, hi = window
        if type(lo) is not int or type(hi) is not int or lo >= hi:
            raise ValueError("Expected increasing work window")
        free, cursor = [], lo
        for start, end in merged:
            start, end = max(lo, start), min(hi, end)
            if start >= end:
                continue
            if cursor < start:
                free.append([cursor, start])
            cursor = max(cursor, end)
        if cursor < hi:
            free.append([cursor, hi])
    return {"conflicts": conflicts, "busy": merged, "free": free}
