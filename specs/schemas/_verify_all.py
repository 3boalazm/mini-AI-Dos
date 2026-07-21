import json, glob
from jsonschema import Draft202012Validator

files = sorted(glob.glob("/home/claude/specs/schemas/*.schema.json"))
print(f"--- {len(files)} schemas in schemas/ ---")
ids = []
for f in files:
    schema = json.load(open(f))
    Draft202012Validator.check_schema(schema)
    ids.append(schema["$id"])
    print(f"PASS: {f.split('/')[-1]}")

dupes = [i for i in ids if ids.count(i) > 1]
print("\nPASS: all $id values unique" if not dupes else f"FAIL: duplicate $id -> {dupes}")
print("\nALL CHECKS PASSED" if not dupes else "SOME CHECKS FAILED")
