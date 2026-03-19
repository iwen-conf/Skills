#!/usr/bin/env python3
from pathlib import Path
import os
import sys

script = Path(__file__).with_name("render_dispatch_report")
os.execv(str(script), [str(script), *sys.argv[1:]])
