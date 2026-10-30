#!/usr/bin/env python3
"""Rewrite the root Package.swift binaryTarget to point at the freshly
published xcframework release, so a local `swift build` can download it.

Usage: python3 scripts/fix_xcf_binarytarget.py <release-tag> <sha256>
"""
import os
import re
import sys

TAG = sys.argv[1]
CK = sys.argv[2]

NEW_URL = "https://github.com/AmatsuZero/openhongguo/releases/download/%s/OpenhongguoCore.xcframework.zip" % TAG

PATH = os.path.join(os.path.dirname(__file__), "..", "Package.swift")

with open(PATH) as f:
    s = f.read()

s = re.sub(
    r'url: "https://github\.com/AmatsuZero/openhongguo/releases/download/xcf-v[^"]+/OpenhongguoCore\.xcframework\.zip"',
    'url: "%s"' % NEW_URL,
    s,
)
s = re.sub(
    r'url: "https://github\.com/beilunyang/openhongguo/releases/download/[^"]+/OpenhongguoCore\.xcframework\.zip"',
    'url: "%s"' % NEW_URL,
    s,
)
s = re.sub(r'checksum: "[0-9a-f]+"', 'checksum: "%s"' % CK, s)
s = s.replace("beilunyang/openhongguo.git", "AmatsuZero/openhongguo.git")

with open(PATH, "w") as f:
    f.write(s)

print("--- updated binaryTarget ---")
for line in s.splitlines():
    if "url:" in line or "checksum:" in line:
        print(line.strip())
