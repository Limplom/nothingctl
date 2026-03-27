#!/usr/bin/env python3
"""nothingctl — thin entry point. All logic lives in the nothing/ package."""

import sys
from pathlib import Path

# Ensure the skill directory is on the path so `nothing` resolves correctly
# when invoked from any working directory.
sys.path.insert(0, str(Path(__file__).parent))

from nothing.cli import main

if __name__ == "__main__":
    main()
