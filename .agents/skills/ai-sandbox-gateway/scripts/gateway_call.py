#!/usr/bin/env python3
"""Call an AI Sandbox Gateway using only the Python standard library."""

from __future__ import annotations

import argparse
import base64
import binascii
import json
import os
import sys
import time
from pathlib import Path
from typing import Any, Optional
from urllib import error, parse, request

TERMINAL_TASK_STATES = {"completed", "failed", "cancelled"}


class GatewayError(Exception):
    pass


class GatewayClient:
    def __init__(
        self,
        base_url: str,
        token: Optional[str],
        timeout: float,
        studio_token: Optional[str] = None,
    ) -> None:
        if not base_url:
            raise GatewayError(
                "gateway URL is required; set AI_SANDBOX_URL or pass --url"
            )
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.studio_token = studio_token
        self.timeout = timeout

    def _endpoint_url(self, endpoint: str) -> str:
        base = parse.urlsplit(self.base_url)
        target = parse.urlsplit(endpoint)
        if target.scheme or target.netloc:
            raise GatewayError("endpoint must be a path, not an absolute URL")

        path = base.path.rstrip("/") + "/" + target.path.lstrip("/")
        query = parse.parse_qsl(base.query, keep_blank_values=True)
        query.extend(parse.parse_qsl(target.query, keep_blank_values=True))
        if self.studio_token:
            query = [(key, value) for key, value in query if key != "studio_token"]
            query.append(("studio_token", self.studio_token))
        return parse.urlunsplit(
            (base.scheme, base.netloc, path, parse.urlencode(query), "")
        )

    def call(
        self,
        endpoint: str,
        method: str = "POST",
        data: Optional[Any] = None,
        require_token: bool = True,
    ) -> dict[str, Any]:
        if not endpoint.startswith("/"):
            endpoint = "/" + endpoint
        if require_token and not self.token:
            raise GatewayError(
                "gateway token is required; configure AI_SANDBOX_CREDENTIALS "
                "or GATEWAY_TOKEN"
            )

        body = None
        headers = {"Accept": "application/json"}
        if data is not None:
            body = json.dumps(data).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if self.token:
            headers["X-Gateway-Token"] = self.token

        req = request.Request(
            self._endpoint_url(endpoint),
            data=body,
            headers=headers,
            method=method.upper(),
        )
        try:
            with request.urlopen(req, timeout=self.timeout) as response:
                raw = response.read()
        except error.HTTPError as exc:
            raw = exc.read()
            detail = _decode_error_body(raw)
            raise GatewayError(f"HTTP {exc.code} for {endpoint}: {detail}") from exc
        except error.URLError as exc:
            raise GatewayError(f"cannot reach gateway: {exc.reason}") from exc
        except TimeoutError as exc:
            raise GatewayError(
                f"gateway request timed out after {self.timeout:g}s"
            ) from exc

        try:
            result = json.loads(raw)
        except (json.JSONDecodeError, UnicodeDecodeError) as exc:
            raise GatewayError(f"gateway returned non-JSON for {endpoint}") from exc
        if not isinstance(result, dict):
            raise GatewayError(f"gateway returned non-object JSON for {endpoint}")
        return result


def _decode_error_body(raw: bytes) -> str:
    text = raw.decode("utf-8", errors="replace").strip()
    if not text:
        return "empty response"
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        return text[:1000]
    if isinstance(payload, dict) and payload.get("error"):
        return str(payload["error"])
    return text[:1000]


def _load_json(value: Optional[str], filename: Optional[str]) -> Optional[Any]:
    if value is not None and filename is not None:
        raise GatewayError("use only one of --data and --data-file")
    if filename is not None:
        try:
            return json.loads(Path(filename).read_text(encoding="utf-8"))
        except OSError as exc:
            raise GatewayError(f"cannot read JSON file {filename}: {exc}") from exc
        except json.JSONDecodeError as exc:
            raise GatewayError(f"invalid JSON in {filename}: {exc}") from exc
    if value is not None:
        try:
            return json.loads(value)
        except json.JSONDecodeError as exc:
            raise GatewayError(f"invalid --data JSON: {exc}") from exc
    return None


def _load_credentials(filename: Optional[str]) -> tuple[Optional[str], Optional[str]]:
    if not filename:
        return None, None
    path = Path(filename).expanduser()
    try:
        raw = path.read_text(encoding="utf-8").strip()
    except OSError as exc:
        raise GatewayError(f"cannot read credentials file {path}: {exc}") from exc
    if not raw:
        raise GatewayError(f"credentials file {path} is empty")

    if not raw.startswith("{"):
        return raw, None
    try:
        credentials = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise GatewayError(f"invalid JSON in credentials file {path}: {exc}") from exc
    if not isinstance(credentials, dict):
        raise GatewayError(f"credentials file {path} must contain a JSON object")

    values: list[Optional[str]] = []
    for key in ("gateway_token", "studio_token"):
        value = credentials.get(key)
        if value is not None and not isinstance(value, str):
            raise GatewayError(f"credentials field {key} must be a string")
        values.append(value.strip() if value else None)
    return values[0], values[1]


def _parse_env(values: list[str]) -> dict[str, str]:
    parsed: dict[str, str] = {}
    for value in values:
        key, separator, item = value.partition("=")
        if not separator or not key:
            raise GatewayError(f"invalid --env value {value!r}; expected KEY=VALUE")
        parsed[key] = item
    return parsed


def _print_json(payload: dict[str, Any]) -> None:
    json.dump(payload, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")


def _response_failed(payload: dict[str, Any]) -> bool:
    return payload.get("success") is False or payload.get("ok") is False


def _client_from_args(args: argparse.Namespace) -> GatewayClient:
    base_url = (
        args.url
        or os.environ.get("AI_SANDBOX_URL")
        or os.environ.get("SANDBOX_BASE_URL")
        or ""
    )
    credentials_file = args.credentials or os.environ.get("AI_SANDBOX_CREDENTIALS")
    file_token, file_studio_token = _load_credentials(credentials_file)
    token = args.token or os.environ.get("GATEWAY_TOKEN") or file_token
    studio_token = (
        args.studio_token
        or os.environ.get("MODELSCOPE_STUDIO_TOKEN")
        or file_studio_token
    )
    return GatewayClient(base_url, token, args.request_timeout, studio_token)


def _run(args: argparse.Namespace) -> int:
    client = _client_from_args(args)

    if args.action == "health":
        result = client.call("/health", method="GET", require_token=False)
        _print_json(result)
        return int(_response_failed(result))

    if args.action == "call":
        data = _load_json(args.data, args.data_file)
        result = client.call(args.endpoint, method=args.method, data=data)
        _print_json(result)
        return int(_response_failed(result))

    if args.action == "upload":
        try:
            content = Path(args.local_path).read_bytes()
        except OSError as exc:
            raise GatewayError(f"cannot read {args.local_path}: {exc}") from exc
        result = client.call(
            "/upload",
            data={
                "path": args.remote_path,
                "content_b64": base64.b64encode(content).decode("ascii"),
            },
        )
        _print_json(result)
        return int(_response_failed(result))

    if args.action == "download":
        result = client.call("/download", data={"path": args.remote_path})
        if _response_failed(result):
            _print_json(result)
            return 1
        try:
            content = base64.b64decode(result["content_b64"], validate=True)
        except (KeyError, ValueError, binascii.Error) as exc:
            raise GatewayError("download response has invalid content_b64") from exc
        destination = Path(args.local_path)
        try:
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(content)
        except OSError as exc:
            raise GatewayError(f"cannot write {destination}: {exc}") from exc
        _print_json(
            {
                "success": True,
                "remote_path": args.remote_path,
                "local_path": str(destination),
                "size": len(content),
            }
        )
        return 0

    if args.action == "task":
        create_body: dict[str, Any] = {
            "command": args.command,
            "timeout": args.timeout,
        }
        if args.cwd:
            create_body["cwd"] = args.cwd
        environment = _parse_env(args.env)
        if environment:
            create_body["env"] = environment

        created = client.call("/task/create", data=create_body)
        if _response_failed(created):
            _print_json(created)
            return 1
        task_id = created.get("task_id")
        if not isinstance(task_id, str) or not task_id:
            raise GatewayError("task/create response has no task_id")

        print(f"task {task_id}: {created.get('status', 'created')}", file=sys.stderr)
        last_status = created.get("status")
        try:
            while True:
                time.sleep(args.interval)
                status = client.call(
                    "/task/status",
                    data={"task_id": task_id},
                )
                current = status.get("status")
                if current != last_status:
                    print(f"task {task_id}: {current}", file=sys.stderr)
                    last_status = current
                if current in TERMINAL_TASK_STATES:
                    _print_json(status)
                    return int(current != "completed" or _response_failed(status))
        except KeyboardInterrupt:
            print(
                f"interrupted locally; remote task {task_id} was not cancelled",
                file=sys.stderr,
            )
            return 130

    raise GatewayError(f"unknown action: {args.action}")


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--url",
        help="gateway base URL (env: AI_SANDBOX_URL or SANDBOX_BASE_URL)",
    )
    parser.add_argument("--token", help="gateway token (prefer credentials file)")
    parser.add_argument(
        "--studio-token",
        help="ModelScope Studio token (prefer credentials file)",
    )
    parser.add_argument(
        "--credentials",
        help="JSON credentials file (env: AI_SANDBOX_CREDENTIALS)",
    )
    parser.add_argument(
        "--request-timeout",
        type=float,
        default=70,
        help="HTTP request timeout in seconds (default: 70)",
    )
    subparsers = parser.add_subparsers(dest="action", required=True)

    subparsers.add_parser("health", help="call the public health endpoint")

    call_parser = subparsers.add_parser("call", help="call any JSON endpoint")
    call_parser.add_argument("endpoint", help="endpoint path, for example /exec")
    call_parser.add_argument("--method", default="POST", choices=("GET", "POST"))
    call_parser.add_argument("--data", help="inline JSON request body")
    call_parser.add_argument("--data-file", help="path to a JSON request body")

    task_parser = subparsers.add_parser("task", help="create and poll a task")
    task_parser.add_argument("command", help="shell command to run")
    task_parser.add_argument("--cwd", help="remote working directory")
    task_parser.add_argument(
        "--env", action="append", default=[], metavar="KEY=VALUE", help="remote env"
    )
    task_parser.add_argument("--timeout", type=int, default=300)
    task_parser.add_argument("--interval", type=float, default=2.0)

    upload_parser = subparsers.add_parser("upload", help="upload a binary file")
    upload_parser.add_argument("local_path")
    upload_parser.add_argument("remote_path")

    download_parser = subparsers.add_parser("download", help="download a binary file")
    download_parser.add_argument("remote_path")
    download_parser.add_argument("local_path")

    return parser


def main() -> int:
    try:
        return _run(_build_parser().parse_args())
    except GatewayError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
