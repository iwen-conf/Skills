#!/usr/bin/env python3
from pathlib import Path
import os
import sys

script = Path(__file__).with_name("cartographer")
os.execv(str(script), [str(script), *sys.argv[1:]])
