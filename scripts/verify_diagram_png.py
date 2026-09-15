#!/usr/bin/env python3
"""Verify the architecture PNG is intact and rendered at the approved 2x size."""

import struct
import sys
from pathlib import Path


def main() -> None:
    path = Path(sys.argv[1])
    data = path.read_bytes()
    if len(data) < 24 or data[:8] != b"\x89PNG\r\n\x1a\n" or data[12:16] != b"IHDR":
        raise SystemExit(f"{path}: not a valid PNG with an IHDR header")
    width, height = struct.unpack(">II", data[16:24])
    if (width, height) != (3200, 2000):
        raise SystemExit(f"{path}: expected readable 3200x2000 output, got {width}x{height}")
    print(f"{path}: valid readable-size PNG ({width}x{height}, {len(data)} bytes)")


if __name__ == "__main__":
    main()
