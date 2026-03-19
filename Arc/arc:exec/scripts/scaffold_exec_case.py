#!/usr/bin/env python3
from pathlib import Path
import os
import sys

script = Path(__file__).with_name("scaffold_exec_case")
os.execv(str(script), [str(script), *sys.argv[1:]])
