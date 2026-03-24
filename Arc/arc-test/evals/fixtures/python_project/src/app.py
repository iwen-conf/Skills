"""Example application module."""


def add(a: int, b: int) -> int:
    """Return the sum of two integers."""
    return a + b


def clamp(v: int, lo: int, hi: int) -> int:
    """Restrict v to the range [lo, hi]."""
    if v < lo:
        return lo
    if v > hi:
        return hi
    return v
