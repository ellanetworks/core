#!/usr/bin/env python3
# SPDX-FileCopyrightText: Ella Networks Inc.
# SPDX-License-Identifier: BUSL-1.1

import argparse
import importlib.metadata
import json
import random

from pycrate_asn1dir import LPP
from pycrate_asn1rt.refobj import ASN1RefType

PYCRATE_VERSION = "0.8.1"

DEFS = LPP.LPP_PDU_Definitions
MESSAGE = DEFS.LPP_Message

BODIES = [
    "requestCapabilities",
    "provideCapabilities",
    "requestAssistanceData",
    "provideAssistanceData",
    "requestLocationInformation",
    "provideLocationInformation",
    "abort",
    "error",
]

CAPTURES = [
    (
        "capture/provide-capabilities-agnss-gps",
        "f0010142087800174027a68050300bf80ea15020701000100720641ec0501f0080",
    ),
]

MAX_LIST = 3
MAX_BITS = 40
MAX_DEPTH = 40


def root_ranges(const):
    if const is None or not const.root:
        return None
    first = const.root[0]
    if isinstance(first, int):
        return first, first
    return first.lb, first.ub


def size_between(const, cap):
    rng = root_ranges(const)
    if rng is None:
        return 0, cap
    lb, ub = rng
    return lb, max(lb, min(ub, lb + cap))


class Generator:
    def __init__(self, rnd, with_ext):
        self.rnd = rnd
        self.with_ext = with_ext

    def value(self, obj, depth=0, in_ext=False):
        kind = obj.TYPE
        if depth > MAX_DEPTH:
            raise RecursionError(obj._name)
        if kind == "INTEGER":
            rng = root_ranges(obj._const_val)
            if rng is None:
                return self.rnd.randint(-1000, 1000)
            lb, ub = rng
            return self.rnd.choice([lb, ub, self.rnd.randint(lb, ub)])
        if kind == "BOOLEAN":
            return self.rnd.random() < 0.5
        if kind == "NULL":
            return 0
        if kind == "ENUMERATED":
            names = list(obj._root)
            if self.with_ext and obj._ext:
                names += list(obj._ext)
            return self.rnd.choice(names)
        if kind == "BIT STRING":
            lb, ub = size_between(obj._const_sz, MAX_BITS)
            n = self.rnd.randint(lb, ub) if ub > lb else lb
            if n == 0 and root_ranges(obj._const_sz) is None:
                n = self.rnd.randint(1, 8)
            return (self.rnd.getrandbits(n) if n else 0, n)
        if kind == "OCTET STRING":
            lb, ub = size_between(obj._const_sz, 6)
            return bytes(self.rnd.getrandbits(8) for _ in range(self.rnd.randint(lb, ub)))
        if kind in ("VisibleString", "PrintableString", "IA5String", "UTF8String"):
            lb, ub = size_between(obj._const_sz, 8)
            n = max(1, self.rnd.randint(lb, ub))
            return "".join(self.rnd.choice("abcdefghijklmnopqrstuvwxyz0123456789") for _ in range(n))
        if kind == "UTCTime":
            return ("26", "09", "28", "12", "00", "00", "Z")
        if kind in ("SEQUENCE", "SET"):
            out = {}
            for name in obj._root:
                comp = obj._cont[name]
                optional = name in (obj._root_opt or [])
                if optional and self.rnd.random() < 0.5:
                    continue
                out[name] = self.value(comp, depth + 1, in_ext)
            if self.with_ext and obj._ext and not in_ext:
                for name in obj._ext:
                    if self.rnd.random() < 0.4:
                        out[name] = self.value(obj._cont[name], depth + 1, True)
            return out
        if kind == "CHOICE":
            names = list(obj._root)
            if self.with_ext and obj._ext and not in_ext and self.rnd.random() < 0.3:
                name = self.rnd.choice(list(obj._ext))
                return (name, self.value(obj._cont[name], depth + 1, True))
            weights = [0.05 if obj._cont[n].TYPE == "NULL" else 1.0 for n in names]
            name = self.rnd.choices(names, weights)[0]
            return (name, self.value(obj._cont[name], depth + 1, in_ext))
        if kind in ("SEQUENCE OF", "SET OF"):
            lb, ub = size_between(obj._const_sz, MAX_LIST)
            n = self.rnd.randint(max(lb, 1) if lb else 0, ub) if ub else lb
            n = max(n, lb)
            return [self.value(obj._cont, depth + 1, in_ext) for _ in range(n)]
        raise NotImplementedError(kind)


def canonical(obj, val):
    kind = obj.TYPE
    if kind == "INTEGER":
        return val
    if kind == "BOOLEAN":
        return bool(val)
    if kind == "NULL":
        return "NULL"
    if kind == "ENUMERATED":
        if val in obj._root:
            return obj._root.index(val)
        return len(obj._root) + list(obj._ext).index(val)
    if kind == "BIT STRING":
        bits, n = val
        return format(bits, "0%db" % n) if n else ""
    if kind == "OCTET STRING":
        return val.hex()
    if kind in ("VisibleString", "PrintableString", "IA5String", "UTF8String"):
        return val
    if kind in ("SEQUENCE", "SET"):
        if obj._ext is None and not obj._root:
            return "NULL"
        if obj._ext is None and len(obj._root) == 1 and obj._root[0] not in (obj._root_opt or []):
            name = obj._root[0]
            return canonical(obj._cont[name], val[name])
        out = []
        for name in obj._root:
            comp = obj._cont[name]
            if name in val:
                out.append(canonical(comp, val[name]))
            elif comp._def is not None:
                out.append(canonical(comp, comp._def))
            else:
                out.append(None)
        return out
    if kind == "CHOICE":
        name, inner = val
        if name in obj._root:
            return [obj._root.index(name), canonical(obj._cont[name], inner)]
        return ["ext"]
    if kind in ("SEQUENCE OF", "SET OF"):
        return [canonical(obj._cont, v) for v in val]
    raise NotImplementedError(kind)


def uses_extensions(obj, val):
    kind = obj.TYPE
    if kind == "ENUMERATED":
        return False
    if kind in ("SEQUENCE", "SET"):
        for name, inner in val.items():
            if name not in obj._root:
                return True
            if uses_extensions(obj._cont[name], inner):
                return True
        return False
    if kind == "CHOICE":
        name, inner = val
        if name not in obj._root:
            return True
        return uses_extensions(obj._cont[name], inner)
    if kind in ("SEQUENCE OF", "SET OF"):
        return any(uses_extensions(obj._cont, v) for v in val)
    return False


def features(obj, val, path, out):
    kind = obj.TYPE
    if kind == "ENUMERATED":
        out.add((path, "enum", val))
    elif kind == "BOOLEAN":
        out.add((path, "bool", bool(val)))
    elif kind in ("SEQUENCE", "SET"):
        for name in obj._root:
            out.add((path, "present", name, name in val))
        for name, inner in val.items():
            out.add((path, "present", name, True))
            features(obj._cont[name], inner, path + "." + name, out)
    elif kind == "CHOICE":
        name, inner = val
        out.add((path, "choice", name))
        features(obj._cont[name], inner, path + "." + name, out)
    elif kind in ("SEQUENCE OF", "SET OF"):
        out.add((path, "len", min(len(val), 2)))
        for inner in val:
            features(obj._cont, inner, path + "[]", out)


def message_value(rnd, gen, body):
    msg = gen.value(MESSAGE)
    ext = MESSAGE._cont["lpp-MessageBody"]._cont["c1"]._cont[body]
    msg["lpp-MessageBody"] = ("c1", (body, gen.value(ext, 2)))
    return msg


def vector(name, msg):
    MESSAGE.set_val(msg)
    raw = MESSAGE.to_uper()
    MESSAGE.from_uper(raw)
    decoded = MESSAGE.get_val()
    return {
        "name": name,
        "hex": raw.hex(),
        "roundTrip": not uses_extensions(MESSAGE, decoded),
        "value": canonical(MESSAGE, decoded),
    }


STRING_KINDS = ("VisibleString", "PrintableString", "IA5String", "UTF8String")


def constraint(const):
    rng = root_ranges(const)
    return None if rng is None else list(rng)


def describe(obj, types):
    tr = obj._typeref
    if isinstance(tr, ASN1RefType):
        name = tr.called[1]
        if name not in types:
            types[name] = None
            types[name] = describe_inline(getattr(DEFS, name.replace("-", "_")), types)
        return {"ref": name}
    return describe_inline(obj, types)


def describe_inline(obj, types):
    kind = obj.TYPE
    if kind in ("SEQUENCE", "SET"):
        opt = obj._root_opt or []
        return {
            "kind": "SEQUENCE",
            "ext": obj._ext is not None,
            "comps": [
                {
                    "name": name,
                    "optional": name in opt and obj._cont[name]._def is None,
                    "default": obj._cont[name]._def is not None,
                    "type": describe(obj._cont[name], types),
                }
                for name in obj._root
            ],
        }
    if kind == "CHOICE":
        return {
            "kind": "CHOICE",
            "ext": obj._ext is not None,
            "alts": [{"name": name, "type": describe(obj._cont[name], types)} for name in obj._root],
        }
    if kind in ("SEQUENCE OF", "SET OF"):
        return {"kind": "SEQUENCE OF", "size": constraint(obj._const_sz), "elem": describe(obj._cont, types)}
    if kind == "INTEGER":
        return {"kind": "INTEGER", "range": constraint(obj._const_val)}
    if kind == "ENUMERATED":
        return {"kind": "ENUMERATED", "root": list(obj._root), "ext": None if obj._ext is None else list(obj._ext)}
    if kind == "BIT STRING":
        names = sorted(obj._cont.items(), key=lambda kv: kv[1]) if obj._cont else []
        return {"kind": "BIT STRING", "size": constraint(obj._const_sz), "names": [n for n, _ in names]}
    if kind == "OCTET STRING":
        return {"kind": "OCTET STRING", "size": constraint(obj._const_sz)}
    if kind in STRING_KINDS:
        return {"kind": kind, "size": constraint(obj._const_sz)}
    if kind in ("BOOLEAN", "NULL"):
        return {"kind": kind}
    raise NotImplementedError(kind)


def schema():
    types = {}
    root = describe_inline(MESSAGE, types)
    types["LPP-Message"] = root
    return {"root": "LPP-Message", "types": types}


def main():
    parser = argparse.ArgumentParser(
        description="Write TS 37.355 LPP-Message conformance vectors encoded by pycrate."
    )
    parser.add_argument("-o", "--output", default="vectors.json")
    parser.add_argument("--schema", default="schema.json")
    parser.add_argument("-n", "--per-body", type=int, default=60)
    parser.add_argument("-c", "--candidates", type=int, default=4000)
    parser.add_argument("-s", "--seed", type=int, default=37355)
    args = parser.parse_args()

    installed = importlib.metadata.version("pycrate")
    if installed != PYCRATE_VERSION:
        parser.error("pycrate %s is installed; the vectors are pinned to %s" % (installed, PYCRATE_VERSION))

    rnd = random.Random(args.seed)
    vectors = []

    for name, hexstr in CAPTURES:
        MESSAGE.from_uper(bytes.fromhex(hexstr))
        decoded = MESSAGE.get_val()
        vectors.append(
            {
                "name": name,
                "hex": hexstr,
                "roundTrip": not uses_extensions(MESSAGE, decoded),
                "value": canonical(MESSAGE, decoded),
            }
        )

    for body in BODIES:
        for with_ext in (False, True):
            seen = set()
            kept = 0
            kind = "ext" if with_ext else "root"
            for _ in range(args.candidates):
                if kept >= args.per_body:
                    break
                gen = Generator(rnd, with_ext)
                msg = message_value(rnd, gen, body)
                MESSAGE.set_val(msg)
                MESSAGE.from_uper(MESSAGE.to_uper())
                found = set()
                features(MESSAGE, MESSAGE.get_val(), "", found)
                if found <= seen:
                    continue
                seen |= found
                vectors.append(vector("random/%s/%s/%03d" % (body, kind, kept), msg))
                kept += 1

    with open(args.output, "w") as f:
        f.write('{"pycrate":%s,"vectors":[\n' % json.dumps(PYCRATE_VERSION))
        f.write(",\n".join(json.dumps(v, separators=(",", ":")) for v in vectors))
        f.write("\n]}\n")

    with open(args.schema, "w") as f:
        json.dump(schema(), f, indent=1, sort_keys=True)
        f.write("\n")


if __name__ == "__main__":
    main()
