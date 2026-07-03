#!/usr/bin/env python3
"""Regenerate the Go GitmojiTable slice literal from the canonical upstream JSON.

Usage (refresh flow — FR-D5 discipline):
  curl -s https://raw.githubusercontent.com/carloscuesta/gitmoji/master/packages/gitmojis/src/gitmojis.json -o /tmp/g.json
  python3 regenerate_table.py /tmp/g.json > gitmoji_table.go.txt
Then paste the body into internal/prompt/gitmoji.go and update gitmojiVerifiedCount /
gitmojiVerifiedDate in the header comment to match.
"""
import json, sys
src = sys.argv[1] if len(sys.argv) > 1 else "gitmojis_canonical.json"
d = json.load(open(src))
g = d["gitmojis"]
def goesc(s): return s.replace("\\", "\\\\").replace('"', '\\"')
print(f"// count={len(g)}")
print("var GitmojiTable = []Gitmoji{")
for x in g:
    print(f"\t{{Emoji: \"{goesc(x['emoji'])}\", Description: \"{goesc(x['description'])}\", Name: \"{goesc(x['name'])}\"}},")
print("}")
