#!/usr/bin/env python3
import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

RESERVED = {"key", "cert"}
CONFIG_TYPES = {"string", "boolean", "enum"}
ACTION_PARAM_TYPES = {"string", "boolean", "select", "textarea"}


def fail(message):
    print(f"ERROR: {message}", file=sys.stderr)
    return False


def warn(message):
    print(f"WARN: {message}", file=sys.stderr)


def run_plugin(path, request, timeout):
    payload = json.dumps(request, separators=(",", ":")).encode()
    try:
        proc = subprocess.run(
            [str(path)],
            input=payload,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError(f"plugin timed out after {timeout}s") from exc
    except OSError as exc:
        raise RuntimeError(f"cannot execute plugin: {exc}") from exc

    if proc.returncode != 0:
        stderr = proc.stderr.decode(errors="replace").strip()
        detail = f"; stderr={stderr!r}" if stderr else ""
        raise RuntimeError(f"plugin exited with code {proc.returncode}{detail}")

    raw = proc.stdout.decode(errors="strict")
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise RuntimeError(
            f"stdout is not exactly one JSON document: {exc}; stdout={raw!r}"
        ) from exc

    if not isinstance(value, dict):
        raise RuntimeError("response must be a JSON object")
    return value, proc.stderr.decode(errors="replace")


def validate_envelope(resp, require_success=True):
    ok = True
    if not isinstance(resp.get("status"), str) or not resp.get("status"):
        ok = fail("response.status must be a non-empty string") and ok
    if not isinstance(resp.get("message"), str):
        ok = fail("response.message must be a string") and ok
    if "result" in resp and resp["result"] is not None and not isinstance(resp["result"], dict):
        ok = fail("response.result must be an object when present") and ok
    if require_success and resp.get("status") != "success":
        ok = fail(f"response status is not success: {resp.get('message', '')}") and ok
    return ok


def validate_param(item, where, allowed_types):
    ok = True
    if not isinstance(item, dict):
        return fail(f"{where} must be an object")
    name = item.get("name")
    if not isinstance(name, str) or not name:
        ok = fail(f"{where}.name must be a non-empty string") and ok
    if name in RESERVED:
        ok = fail(f"{where}.name uses reserved host field {name!r}") and ok
    typ = item.get("type")
    if not isinstance(typ, str) or not typ:
        ok = fail(f"{where}.type must be a non-empty string") and ok
    elif typ not in allowed_types:
        warn(f"{where}.type={typ!r} is not in the AllinSSL 1.1.3 stable UI subset {sorted(allowed_types)}")
    desc = item.get("description")
    if not isinstance(desc, str) or not desc:
        ok = fail(f"{where}.description must be a non-empty string") and ok
    if not isinstance(item.get("required"), bool):
        ok = fail(f"{where}.required must be boolean") and ok
    if "options" in item:
        options = item["options"]
        if not isinstance(options, list) or not all(isinstance(x, dict) for x in options):
            ok = fail(f"{where}.options must be an array of objects for the 1.1.3 backend type") and ok
    return ok


def validate_metadata(resp):
    ok = validate_envelope(resp, require_success=True)
    meta = resp.get("result")
    if not isinstance(meta, dict):
        fail("metadata response.result must be an object")
        return False, None

    for field in ("name", "description", "version", "author"):
        if not isinstance(meta.get(field), str) or not meta.get(field):
            ok = fail(f"metadata.{field} must be a non-empty string") and ok

    config = meta.get("config", [])
    if isinstance(config, dict):
        ok = fail("metadata.config uses legacy map form; use an array of parameter objects") and ok
        config = []
    elif not isinstance(config, list):
        ok = fail("metadata.config must be an array when present") and ok
        config = []

    config_names = set()
    for i, item in enumerate(config):
        ok = validate_param(item, f"metadata.config[{i}]", CONFIG_TYPES) and ok
        if isinstance(item, dict) and isinstance(item.get("name"), str):
            name = item["name"]
            if name in config_names:
                ok = fail(f"duplicate config field {name!r}") and ok
            config_names.add(name)

    actions = meta.get("actions")
    if not isinstance(actions, list) or not actions:
        fail("metadata.actions must be a non-empty array")
        return False, meta

    action_names = set()
    for i, action in enumerate(actions):
        where = f"metadata.actions[{i}]"
        if not isinstance(action, dict):
            ok = fail(f"{where} must be an object") and ok
            continue
        name = action.get("name")
        if not isinstance(name, str) or not name:
            ok = fail(f"{where}.name must be a non-empty string") and ok
        elif name in action_names:
            ok = fail(f"duplicate action name {name!r}") and ok
        else:
            action_names.add(name)
        if not isinstance(action.get("description"), str) or not action.get("description"):
            ok = fail(f"{where}.description must be a non-empty string") and ok

        params = action.get("params", [])
        if isinstance(params, dict):
            ok = fail(f"{where}.params uses legacy map form; use an array of parameter objects") and ok
            params = []
        elif not isinstance(params, list):
            ok = fail(f"{where}.params must be an array when present") and ok
            params = []

        param_names = set()
        for j, item in enumerate(params):
            ok = validate_param(item, f"{where}.params[{j}]", ACTION_PARAM_TYPES) and ok
            if isinstance(item, dict) and isinstance(item.get("name"), str):
                pname = item["name"]
                if pname in param_names:
                    ok = fail(f"duplicate action param {pname!r} in action {name!r}") and ok
                param_names.add(pname)
        overlap = config_names & param_names
        if overlap:
            warn(
                f"action {name!r} params overlap access config fields {sorted(overlap)}; "
                "AllinSSL 1.1.3 action params overwrite config values"
            )

    return ok, meta


def main():
    parser = argparse.ArgumentParser(description="Validate an AllinSSL 1.1.3 executable plugin protocol")
    parser.add_argument("plugin", help="path to the executable plugin")
    parser.add_argument("--timeout", type=float, default=10.0, help="per-process timeout in seconds")
    parser.add_argument("--action", help="optional action to invoke after metadata validation")
    parser.add_argument("--params", default="{}", help="JSON object passed to the action")
    parser.add_argument("--expect-error", action="store_true", help="expect the action response to be non-success")
    args = parser.parse_args()

    path = Path(args.plugin).expanduser().resolve()
    if not path.is_file():
        return 2 if not fail(f"plugin file not found: {path}") else 2
    if os.name != "nt" and not os.access(path, os.X_OK):
        return 2 if not fail(f"plugin is not executable: {path}") else 2

    try:
        metadata_resp, metadata_stderr = run_plugin(path, {"action": "get_metadata"}, args.timeout)
    except RuntimeError as exc:
        fail(str(exc))
        return 2

    ok, meta = validate_metadata(metadata_resp)
    if metadata_stderr.strip():
        warn("plugin wrote to stderr during get_metadata; AllinSSL does not expose this as a reliable log channel")

    if args.action:
        try:
            params = json.loads(args.params)
        except json.JSONDecodeError as exc:
            fail(f"--params is not valid JSON: {exc}")
            return 2
        if not isinstance(params, dict):
            fail("--params must decode to a JSON object")
            return 2

        declared = {a.get("name") for a in (meta or {}).get("actions", []) if isinstance(a, dict)}
        if args.action not in declared:
            ok = fail(f"action {args.action!r} is not declared in metadata") and ok
        else:
            try:
                action_resp, action_stderr = run_plugin(
                    path,
                    {"action": args.action, "params": params},
                    args.timeout,
                )
            except RuntimeError as exc:
                fail(str(exc))
                return 2

            if action_stderr.strip():
                warn("plugin wrote to stderr during action; AllinSSL 1.1.3 discards action stderr")
            envelope_ok = validate_envelope(action_resp, require_success=not args.expect_error)
            if args.expect_error and action_resp.get("status") == "success":
                envelope_ok = fail("action returned success but --expect-error was requested") and envelope_ok
            ok = envelope_ok and ok

    if ok:
        print("OK: plugin protocol is compatible with the checked AllinSSL 1.1.3 rules")
        return 0
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
