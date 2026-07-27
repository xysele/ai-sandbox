#!/usr/bin/env python3
"""Deploy the current Git revision to a ModelScope.ai Docker Studio."""

from __future__ import annotations

import argparse
import http.cookiejar
import json
import os
import re
import secrets
import socket
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Dict, Optional
from urllib import error, parse, request

MODELSCOPE_ENDPOINT = "https://www.modelscope.ai"
MODELSCOPE_API_ROOT = "/api"
DEFAULT_STUDIO_NAME = "ai-sandbox-go"
DEFAULT_BRANCH = "master"
DEFAULT_SDK_TYPE = "docker"
DEFAULT_VISIBILITY = 1  # Public. The gateway still requires GATEWAY_TOKEN.
DEFAULT_WAIT_TIMEOUT = 900
DEFAULT_POLL_INTERVAL = 10
REQUEST_TIMEOUT = 30
REPO_ROOT = Path(__file__).resolve().parent
TOKEN_FILE = REPO_ROOT / "cs_token.txt"
TERMINAL_FAILURE_STATES = {"Failed", "Error", "DeployFailed", "CreateFailed"}
NON_FATAL_RESTART_MESSAGES = ("data is changing", "Please wait", "数据变更中")
NAME_PATTERN = re.compile(r"^[A-Za-z0-9._-]+$")


class DeploymentError(RuntimeError):
    """A deployment failure with a user-facing message."""


class ModelScopeAPIError(DeploymentError):
    def __init__(self, status: int, code: Any, message: str) -> None:
        self.status = status
        self.code = code
        self.message = message
        super().__init__(
            f"ModelScope API error (HTTP {status}, code={code}): {message}"
        )


class ModelScopeClient:
    """Small cookie-authenticated client for the current ModelScope Studio API."""

    def __init__(self, endpoint: str = MODELSCOPE_ENDPOINT) -> None:
        self.endpoint = endpoint.rstrip("/")
        self.cookies = http.cookiejar.CookieJar()
        self.opener = request.build_opener(
            request.ProxyHandler(),
            request.HTTPCookieProcessor(self.cookies),
        )
        self.username: Optional[str] = None

    def _csrf_token(self) -> Optional[str]:
        for cookie in self.cookies:
            if cookie.name.lower() == "csrf_token":
                return parse.unquote(cookie.value)
        return None

    def request(
        self,
        method: str,
        path: str,
        body: Optional[Dict[str, Any]] = None,
    ) -> Any:
        url = f"{self.endpoint}{MODELSCOPE_API_ROOT}{path}"
        payload = None
        headers = {
            "Accept": "application/json",
            "User-Agent": "ai-sandbox-go-deploy/1.0",
        }
        if body is not None:
            payload = json.dumps(body, ensure_ascii=False).encode("utf-8")
            headers["Content-Type"] = "application/json"

        if method.upper() in {"POST", "PUT", "PATCH", "DELETE"}:
            csrf = self._csrf_token()
            if csrf:
                headers["X-CSRF-TOKEN"] = csrf

        req = request.Request(
            url,
            data=payload,
            headers=headers,
            method=method.upper(),
        )
        status = 0
        raw = b""
        try:
            with self.opener.open(req, timeout=REQUEST_TIMEOUT) as response:
                status = response.status
                raw = response.read()
        except error.HTTPError as exc:
            status = exc.code
            raw = exc.read()
        except (error.URLError, TimeoutError, socket.timeout, OSError) as exc:
            reason = getattr(exc, "reason", exc)
            raise DeploymentError(f"Cannot reach {self.endpoint}: {reason}") from exc

        try:
            response_data = json.loads(raw.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            preview = raw.decode("utf-8", errors="replace")[:500]
            raise DeploymentError(
                f"ModelScope returned non-JSON data for {method.upper()} {path} "
                f"(HTTP {status}): {preview}"
            ) from exc

        if not isinstance(response_data, dict):
            raise DeploymentError(
                f"ModelScope returned invalid JSON for {method.upper()} {path}"
            )

        success = response_data.get("Success", 200 <= status < 300)
        if not 200 <= status < 300 or success is False:
            raise ModelScopeAPIError(
                status,
                response_data.get("Code", "unknown"),
                str(response_data.get("Message") or "unknown error"),
            )
        return response_data.get("Data")

    def login(self, access_key: str) -> Dict[str, Any]:
        data = self.request("POST", "/v1/login", {"AccessToken": access_key}) or {}
        if not isinstance(data, dict):
            raise DeploymentError("ModelScope login returned an invalid response")
        username = data.get("Username") or data.get("username")
        if not username:
            raise DeploymentError("ModelScope login succeeded but returned no username")
        self.username = str(username)
        return data

    def get_studio(self, namespace: str, studio_name: str) -> Optional[Dict[str, Any]]:
        try:
            data = self.request("GET", _studio_path(namespace, studio_name))
        except ModelScopeAPIError as exc:
            if exc.status == 404 or str(exc.code) == "10011402001":
                return None
            raise
        if data is None:
            return None
        if not isinstance(data, dict):
            raise DeploymentError("Studio detail returned an invalid response")
        return data

    def get_status(self, namespace: str, studio_name: str) -> Dict[str, Any]:
        data = (
            self.request("GET", f"{_studio_path(namespace, studio_name)}/status") or {}
        )
        if not isinstance(data, dict):
            raise DeploymentError("Studio status returned an invalid response")
        return data

    def get_default_instance_type_id(self) -> int:
        data = self.request("GET", "/v1/studios/free_instance") or {}
        instances = data.get("FreeInstanceType") or []
        if not instances:
            raise DeploymentError("ModelScope.ai returned no free Studio instance type")
        return int(instances[0]["Id"])

    def get_default_sdk_version(self, sdk_type: str) -> str:
        # Docker Studios build their environment from Dockerfile and currently
        # advertise no SDK versions on modelscope.ai.
        if sdk_type == "docker":
            return ""
        data = self.request("GET", f"/v1/studios/sdk-version/{sdk_type}") or {}
        versions = data.get("Versions") or []
        for version in versions:
            if version.get("Tag") == "default" and not version.get("Hidden"):
                return str(version.get("Version") or "")
        for version in versions:
            if not version.get("Hidden"):
                return str(version.get("Version") or "")
        raise DeploymentError(
            f"ModelScope.ai returned no usable SDK version for {sdk_type}"
        )

    def create_studio(
        self,
        namespace: str,
        studio_name: str,
        sdk_type: str,
        sdk_version: str,
        instance_type_id: int,
        visibility: int,
    ) -> None:
        self.request(
            "POST",
            "/v1/studios",
            {
                "Path": namespace,
                "Name": studio_name,
                "Visibility": visibility,
                "DeployedByUser": False,
                "InstanceTypeId": instance_type_id,
                "InstanceNumber": 1,
                "SdkType": sdk_type,
                "SdkVersion": sdk_version,
            },
        )

    def list_envs(self, namespace: str, studio_name: str) -> list[Dict[str, Any]]:
        data = self.request("GET", f"{_studio_path(namespace, studio_name)}/envs") or {}
        values = data.get("EnvironmentVariables") or []
        return [dict(value) for value in values]

    def set_env(self, namespace: str, studio_name: str, name: str, value: str) -> str:
        current = None
        for item in self.list_envs(namespace, studio_name):
            item_name = item.get("VariableName") or item.get("Name")
            if item_name == name:
                current = item
                break

        payload: Dict[str, Any] = {
            "Operation": "add",
            "VariableName": name,
            "VariableValue": value,
        }
        action = "added"
        if current is not None:
            variable_id = current.get("VariableId") or current.get("Id")
            if variable_id is None:
                raise DeploymentError(
                    f"Existing environment variable {name} has no VariableId"
                )
            payload.update({"Operation": "modify", "VariableId": variable_id})
            action = "updated"

        self.request(
            "PUT",
            f"{_studio_path(namespace, studio_name)}/envs",
            payload,
        )
        return action

    def reset_restart(self, namespace: str, studio_name: str) -> None:
        try:
            self.request(
                "PUT",
                f"{_studio_path(namespace, studio_name)}/reset_restart",
                {},
            )
        except ModelScopeAPIError as exc:
            if any(text in exc.message for text in NON_FATAL_RESTART_MESSAGES):
                return
            raise

    def get_studio_token(self) -> str:
        data = self.request("GET", "/v1/studios/token") or {}
        return str(data.get("Token") or "")


def _studio_path(namespace: str, studio_name: str) -> str:
    namespace_segment = parse.quote(namespace, safe="")
    studio_segment = parse.quote(studio_name, safe="")
    return f"/v1/studio/{namespace_segment}/{studio_segment}"


def _validate_name(label: str, value: str) -> str:
    if not value or not NAME_PATTERN.fullmatch(value):
        raise DeploymentError(
            f"{label} must contain only letters, digits, dot, underscore, or hyphen"
        )
    return value


def _read_access_key() -> str:
    access_key = os.environ.get("MODELSCOPE_ACCESS_KEY") or os.environ.get(
        "MODELSCOPE_API_KEY"
    )
    if not access_key:
        raise DeploymentError(
            "Set MODELSCOPE_ACCESS_KEY to a full ModelScope.ai access token "
            "(MODELSCOPE_API_KEY is also accepted for compatibility)"
        )
    return access_key.strip()


def _read_gateway_token() -> tuple[str, str]:
    configured = os.environ.get("GATEWAY_TOKEN")
    if configured:
        return configured, "GATEWAY_TOKEN"
    if TOKEN_FILE.is_file():
        existing = TOKEN_FILE.read_text(encoding="utf-8").strip()
        if existing:
            return existing, str(TOKEN_FILE)
    return secrets.token_urlsafe(32), "generated"


def _save_gateway_token(token: str) -> None:
    TOKEN_FILE.write_text(token, encoding="utf-8")
    TOKEN_FILE.chmod(0o600)


def _run_git(args: list[str], env: Optional[Dict[str, str]] = None) -> str:
    completed = subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        env=env,
        text=True,
        capture_output=True,
    )
    if completed.returncode != 0:
        command = "git " + " ".join(args)
        raise DeploymentError(
            f"{command} failed ({completed.returncode})\n"
            f"stdout:\n{completed.stdout}\nstderr:\n{completed.stderr}"
        )
    return completed.stdout.strip()


def _check_worktree() -> str:
    root = Path(_run_git(["rev-parse", "--show-toplevel"])).resolve()
    if root != REPO_ROOT:
        raise DeploymentError(f"deploy.py is in {REPO_ROOT}, but Git root is {root}")
    dirty = _run_git(["status", "--porcelain"])
    if dirty:
        preview = "\n".join(dirty.splitlines()[:20])
        raise DeploymentError(
            "Git worktree is not clean. deploy.py pushes the current commit, so "
            "uncommitted files would not be deployed. Commit or stash these "
            "changes first:\n" + preview
        )
    return _run_git(["rev-parse", "HEAD"])


def _git_auth_env(access_key: str) -> Dict[str, str]:
    env = os.environ.copy()
    env.update(
        {
            "GIT_TERMINAL_PROMPT": "0",
            "MODELSCOPE_GIT_TOKEN": access_key,
        }
    )
    return env


def _git_auth_config() -> list[str]:
    credential_helper = (
        '!f() { printf "%s\\n" "username=oauth2" "password=$MODELSCOPE_GIT_TOKEN"; }; f'
    )
    return [
        "-c",
        "credential.helper=",
        "-c",
        f"credential.helper={credential_helper}",
        "-c",
        "credential.useHttpPath=true",
    ]


def _remote_branch_head(
    git_url: str,
    branch: str,
    access_key: str,
) -> Optional[str]:
    output = _run_git(
        [
            *_git_auth_config(),
            "ls-remote",
            "--heads",
            git_url,
            f"refs/heads/{branch}",
        ],
        env=_git_auth_env(access_key),
    )
    if not output:
        return None
    fields = output.splitlines()[0].split()
    if len(fields) != 2:
        raise DeploymentError(f"Git returned an invalid remote ref: {output}")
    return fields[0]


def _fetch_remote_branch(
    git_url: str,
    branch: str,
    access_key: str,
) -> str:
    _run_git(
        [
            *_git_auth_config(),
            "fetch",
            "--no-tags",
            git_url,
            f"refs/heads/{branch}",
        ],
        env=_git_auth_env(access_key),
    )
    return _run_git(["rev-parse", "FETCH_HEAD"])


def _is_ancestor(ancestor: str, descendant: str) -> bool:
    completed = subprocess.run(
        ["git", "merge-base", "--is-ancestor", ancestor, descendant],
        cwd=REPO_ROOT,
        text=True,
        capture_output=True,
    )
    if completed.returncode in {0, 1}:
        return completed.returncode == 0
    raise DeploymentError(
        "git merge-base failed "
        f"({completed.returncode})\nstdout:\n{completed.stdout}\n"
        f"stderr:\n{completed.stderr}"
    )


def _create_deployment_commit(local_head: str, remote_head: str) -> str:
    tree = _run_git(["rev-parse", f"{local_head}^{{tree}}"])
    short_head = _run_git(["rev-parse", "--short", local_head])
    author_name = _run_git(["show", "-s", "--format=%an", local_head])
    author_email = _run_git(["show", "-s", "--format=%ae", local_head])
    identity_env = os.environ.copy()
    identity_env.update(
        {
            "GIT_AUTHOR_NAME": author_name,
            "GIT_AUTHOR_EMAIL": author_email,
            "GIT_COMMITTER_NAME": author_name,
            "GIT_COMMITTER_EMAIL": author_email,
        }
    )
    return _run_git(
        [
            "commit-tree",
            tree,
            "-p",
            remote_head,
            "-p",
            local_head,
            "-m",
            f"Deploy {short_head} to ModelScope Studio",
        ],
        env=identity_env,
    )


def _push_ref(
    git_url: str,
    source: str,
    branch: str,
    access_key: str,
) -> None:
    _run_git(
        [
            *_git_auth_config(),
            "push",
            git_url,
            f"{source}:refs/heads/{branch}",
        ],
        env=_git_auth_env(access_key),
    )


def _push_code(
    namespace: str,
    studio_name: str,
    access_key: str,
    branch: str,
) -> None:
    git_url = f"{MODELSCOPE_ENDPOINT}/studios/{namespace}/{studio_name}.git"
    local_head = _run_git(["rev-parse", "HEAD"])
    local_tree = _run_git(["rev-parse", f"{local_head}^{{tree}}"])
    last_error: Optional[DeploymentError] = None
    for attempt in range(1, 4):
        try:
            if _remote_branch_head(git_url, branch, access_key) is None:
                source = local_head
            else:
                remote_head = _fetch_remote_branch(git_url, branch, access_key)
                remote_tree = _run_git(["rev-parse", f"{remote_head}^{{tree}}"])
                if remote_tree == local_tree:
                    print("  Remote branch already has the same files as local HEAD.")
                    return
                if _is_ancestor(remote_head, local_head):
                    source = local_head
                else:
                    print(
                        "  Preserving the remote branch history in a deployment "
                        "merge commit."
                    )
                    source = _create_deployment_commit(local_head, remote_head)
            _push_ref(git_url, source, branch, access_key)
            return
        except DeploymentError as exc:
            last_error = exc
            if attempt < 3:
                time.sleep(attempt * 2)
    if last_error is not None:
        raise last_error


def _wait_for_studio(
    client: ModelScopeClient,
    namespace: str,
    studio_name: str,
    timeout: int,
    interval: int,
) -> Dict[str, Any]:
    deadline = time.time() + timeout
    last_status: Optional[str] = None
    last_data: Dict[str, Any] = {}
    while time.time() < deadline:
        last_data = client.get_status(namespace, studio_name)
        status = str(last_data.get("Status") or "Unknown")
        if status != last_status:
            print(f"  Studio status: {status}")
            last_status = status
        if status == "Running":
            return last_data
        if status in TERMINAL_FAILURE_STATES:
            raise DeploymentError(f"Studio entered failure state: {status}")
        time.sleep(interval)
    raise DeploymentError(
        f"Timed out after {timeout}s waiting for Running; last status: {last_status}"
    )


def _wait_for_detail(
    client: ModelScopeClient, namespace: str, studio_name: str
) -> Dict[str, Any]:
    for attempt in range(1, 11):
        detail = client.get_studio(namespace, studio_name)
        if detail is not None:
            return detail
        time.sleep(min(attempt, 5))
    raise DeploymentError(
        "Studio creation returned success, but the Studio is still unavailable"
    )


def _platform_check(sdk_type: str) -> int:
    client = ModelScopeClient()
    addresses = sorted(
        {
            item[4][0]
            for item in socket.getaddrinfo(
                "www.modelscope.ai", 443, type=socket.SOCK_STREAM
            )
        }
    )
    instance_data = client.request("GET", "/v1/studios/free_instance") or {}
    version_data = client.request("GET", f"/v1/studios/sdk-version/{sdk_type}") or {}
    print(
        json.dumps(
            {
                "endpoint": MODELSCOPE_ENDPOINT,
                "addresses": addresses,
                "sdk_type": sdk_type,
                "sdk_versions": version_data.get("Versions") or [],
                "free_instances": instance_data.get("FreeInstanceType") or [],
                "note": (
                    "Docker uses Dockerfile and does not require an SDK version."
                    if sdk_type == "docker"
                    else None
                ),
            },
            ensure_ascii=False,
            indent=2,
        )
    )
    return 0


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Deploy the current Git revision to ModelScope.ai Studio."
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="check public ModelScope.ai Studio capabilities without logging in",
    )
    parser.add_argument(
        "--namespace",
        default=os.environ.get("MODELSCOPE_NAMESPACE"),
        help="Studio namespace; defaults to the username returned by login",
    )
    parser.add_argument(
        "--studio-name",
        default=os.environ.get("MODELSCOPE_STUDIO_NAME", DEFAULT_STUDIO_NAME),
    )
    parser.add_argument(
        "--sdk-type",
        choices=("docker", "gradio", "streamlit", "static"),
        default=os.environ.get("MODELSCOPE_SDK_TYPE", DEFAULT_SDK_TYPE),
    )
    parser.add_argument(
        "--sdk-version",
        default=os.environ.get("MODELSCOPE_SDK_VERSION"),
        help="override the platform SDK version; Docker normally leaves this empty",
    )
    parser.add_argument(
        "--instance-type-id",
        type=int,
        default=(
            int(os.environ["MODELSCOPE_INSTANCE_TYPE_ID"])
            if os.environ.get("MODELSCOPE_INSTANCE_TYPE_ID")
            else None
        ),
    )
    parser.add_argument(
        "--visibility",
        type=int,
        default=int(os.environ.get("MODELSCOPE_VISIBILITY", DEFAULT_VISIBILITY)),
        help="ModelScope visibility value; default 1 (public)",
    )
    parser.add_argument(
        "--branch",
        default=os.environ.get("MODELSCOPE_BRANCH", DEFAULT_BRANCH),
        help="remote Studio branch; default master",
    )
    parser.add_argument(
        "--no-wait",
        action="store_true",
        help="trigger reset_restart but do not wait for Running",
    )
    parser.add_argument("--wait-timeout", type=int, default=DEFAULT_WAIT_TIMEOUT)
    parser.add_argument("--poll-interval", type=int, default=DEFAULT_POLL_INTERVAL)
    return parser


def deploy(args: argparse.Namespace) -> int:
    commit = _check_worktree()
    access_key = _read_access_key()
    gateway_token, token_source = _read_gateway_token()

    print(f"ModelScope endpoint: {MODELSCOPE_ENDPOINT}")
    print(f"Local commit: {commit[:12]}")
    print("Logging in...")
    client = ModelScopeClient()
    login_data = client.login(access_key)

    namespace = _validate_name(
        "namespace", args.namespace or str(login_data.get("Username") or "")
    )
    studio_name = _validate_name("studio name", args.studio_name)
    branch = _validate_name("branch", args.branch)

    detail = client.get_studio(namespace, studio_name)
    created = False
    if detail is None:
        sdk_version = (
            args.sdk_version
            if args.sdk_version is not None
            else client.get_default_sdk_version(args.sdk_type)
        )
        instance_type_id = (
            args.instance_type_id or client.get_default_instance_type_id()
        )
        print(
            f"Creating Studio {namespace}/{studio_name} "
            f"({args.sdk_type}, instance type {instance_type_id})..."
        )
        client.create_studio(
            namespace,
            studio_name,
            args.sdk_type,
            sdk_version,
            instance_type_id,
            args.visibility,
        )
        created = True
        detail = _wait_for_detail(client, namespace, studio_name)
    else:
        existing_sdk = str(detail.get("SdkType") or "")
        if existing_sdk and existing_sdk.lower() != args.sdk_type.lower():
            raise DeploymentError(
                f"Existing Studio uses SDK type {existing_sdk}, not {args.sdk_type}"
            )
        print(f"Reusing Studio {namespace}/{studio_name}.")

    if created:
        print(
            f"Pushing HEAD to {branch}; preserving the new Studio's generated "
            "initial commit..."
        )
    else:
        print(f"Pushing HEAD to {branch}...")
    _push_code(
        namespace,
        studio_name,
        access_key,
        branch,
    )

    action = client.set_env(namespace, studio_name, "GATEWAY_TOKEN", gateway_token)
    _save_gateway_token(gateway_token)
    print(f"GATEWAY_TOKEN {action}; local copy: {TOKEN_FILE.name} ({token_source}).")

    print("Triggering reset_restart...")
    client.reset_restart(namespace, studio_name)
    status = {"Status": "restart requested"}
    if not args.no_wait:
        status = _wait_for_studio(
            client,
            namespace,
            studio_name,
            args.wait_timeout,
            args.poll_interval,
        )

    detail = client.get_studio(namespace, studio_name) or detail
    studio_token = client.get_studio_token()
    studio_page = f"{MODELSCOPE_ENDPOINT}/studios/{namespace}/{studio_name}"
    independent_url = str(detail.get("IndependentUrl") or "")

    print("\nDeployment finished.")
    print(f"Studio: {studio_page}")
    print(f"Status: {status.get('Status')}")
    if independent_url:
        print(f"Runtime base URL: {independent_url}")
        if studio_token:
            health_url = (
                independent_url.rstrip("/")
                + "/health?"
                + parse.urlencode({"studio_token": studio_token})
            )
            print(f"Tokenized health URL: {health_url}")
    else:
        print("Runtime URL is not available yet; check the Studio page after startup.")
    print(f"Gateway token is stored in {TOKEN_FILE} and was not printed.")
    return 0


def main() -> int:
    args = _build_parser().parse_args()
    try:
        if args.check:
            return _platform_check(args.sdk_type)
        return deploy(args)
    except (DeploymentError, ValueError) as exc:
        print(f"Deployment failed: {exc}", file=sys.stderr)
        return 1
    except KeyboardInterrupt:
        print("Deployment interrupted.", file=sys.stderr)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
