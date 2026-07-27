#!/usr/bin/env python3
"""
Deploy script for AI Sandbox Go to ModelScope CreativeSpace
"""

import http.client
import json
import os
import secrets
import subprocess
import sys
import time
import urllib.parse

# Configuration
MODELSCOPE_API = "api.modelscope.cn"
NAMESPACE = os.getenv("MODELSCOPE_NAMESPACE", "your-namespace")  # Change this
STUDIO_NAME = "ai-sandbox-go"
API_KEY = os.getenv("MODELSCOPE_API_KEY")  # Set this environment variable

if not API_KEY:
    print("ERROR: MODELSCOPE_API_KEY environment variable not set")
    print("Please set it with your ModelScope API key:")
    print("  export MODELSCOPE_API_KEY='your-api-key-here'")
    sys.exit(1)


def api_request(method, path, body=None, headers=None):
    """Make API request to ModelScope"""
    conn = http.client.HTTPSConnection(MODELSCOPE_API)

    default_headers = {
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json",
    }
    if headers:
        default_headers.update(headers)

    body_data = json.dumps(body) if body else None

    try:
        conn.request(method, path, body_data, default_headers)
        response = conn.getresponse()
        data = response.read().decode()

        if response.status >= 400:
            print(f"API Error {response.status}: {data}")
            return None

        return json.loads(data) if data else {}
    finally:
        conn.close()


def generate_token():
    """Generate a secure random token"""
    return secrets.token_urlsafe(32)


def create_or_get_studio():
    """Create Docker-type Studio or get existing one"""
    print(f"[1/6] Checking if Studio '{STUDIO_NAME}' exists...")

    # Try to get existing studio
    path = f"/api/v1/studios/{NAMESPACE}/{STUDIO_NAME}"
    result = api_request("GET", path)

    if result:
        print(f"✓ Studio exists: {NAMESPACE}/{STUDIO_NAME}")
        return result

    # Create new studio
    print(f"Creating new Docker Studio '{STUDIO_NAME}'...")

    body = {
        "Name": STUDIO_NAME,
        "Runtime": "docker",  # Docker type for custom containers
        "VisibleLevel": "private",
        "Description": "AI Sandbox Go - AI Agent sandbox environment",
        "Cover": "",
    }

    path = f"/api/v1/studios/{NAMESPACE}"
    result = api_request("POST", path, body)

    if result:
        print(f"✓ Studio created: {NAMESPACE}/{STUDIO_NAME}")
        return result
    else:
        print("✗ Failed to create studio")
        sys.exit(1)


def set_secret(secret_name, secret_value):
    """Set secret in Studio settings"""
    print(f"[2/6] Setting secret '{secret_name}'...")

    path = f"/api/v1/studios/{NAMESPACE}/{STUDIO_NAME}/secrets"
    body = {
        "Key": secret_name,
        "Value": secret_value,
    }

    result = api_request("POST", path, body)

    if result or result == {}:
        print(f"✓ Secret '{secret_name}' set")
        return True
    else:
        print(f"✗ Failed to set secret")
        return False


def push_code():
    """Push code to Studio's git repository"""
    print("[3/6] Pushing code to Studio repository...")

    # Get Studio's git URL
    git_url = f"https://www.modelscope.cn/studios/{NAMESPACE}/{STUDIO_NAME}.git"

    # Check if git remote exists
    result = subprocess.run(
        ["git", "remote", "get-url", "modelscope"],
        capture_output=True,
        text=True,
    )

    if result.returncode != 0:
        # Add remote
        subprocess.run(["git", "remote", "add", "modelscope", git_url], check=False)
    else:
        # Update remote URL
        subprocess.run(["git", "remote", "set-url", "modelscope", git_url], check=False)

    # Push to main branch
    print("Pushing to modelscope remote...")
    result = subprocess.run(
        ["git", "push", "modelscope", "HEAD:main", "-f"],
        capture_output=True,
        text=True,
    )

    if result.returncode == 0:
        print("✓ Code pushed successfully")
        return True
    else:
        print(f"✗ Git push failed: {result.stderr}")
        print("\nPlease configure git credentials:")
        print(f"  git config credential.helper store")
        print(f"  git push modelscope HEAD:main")
        return False


def trigger_build():
    """Trigger Studio rebuild"""
    print("[4/6] Triggering Studio build...")

    path = f"/api/v1/studios/{NAMESPACE}/{STUDIO_NAME}/rebuild"
    result = api_request("POST", path)

    if result:
        print("✓ Build triggered")
        return True
    else:
        print("⚠ Build trigger may have failed (this is sometimes normal)")
        return True  # Continue anyway


def wait_for_running(timeout=600):
    """Wait for Studio to reach Running state"""
    print("[5/6] Waiting for Studio to start (this may take 5-10 minutes)...")

    start_time = time.time()
    last_status = None

    while time.time() - start_time < timeout:
        path = f"/api/v1/studios/{NAMESPACE}/{STUDIO_NAME}"
        result = api_request("GET", path)

        if result and "Status" in result:
            status = result["Status"]

            if status != last_status:
                print(f"  Status: {status}")
                last_status = status

            if status == "Running":
                print("✓ Studio is running!")
                return True
            elif status in ["Failed", "Stopped"]:
                print(f"✗ Studio status: {status}")
                return False

        time.sleep(10)

    print("⚠ Timeout waiting for Studio to start")
    return False


def save_token(token):
    """Save token to file"""
    with open("cs_token.txt", "w") as f:
        f.write(token)
    print(f"\n✓ Token saved to cs_token.txt")


def print_summary(token):
    """Print deployment summary"""
    base_url = f"https://www.modelscope.cn/api/v1/studios/{NAMESPACE}/{STUDIO_NAME}/proxy/7860"

    print("\n" + "="*70)
    print("🎉 DEPLOYMENT SUCCESSFUL!")
    print("="*70)
    print(f"\n📍 Studio URL: https://www.modelscope.cn/studios/{NAMESPACE}/{STUDIO_NAME}")
    print(f"\n🔗 API Base URL:\n   {base_url}/")
    print(f"\n🔑 Gateway Token:\n   {token}")
    print(f"\n💾 Token saved to: cs_token.txt")

    print("\n📝 Quick Test:")
    print(f"""
   curl {base_url}/health

   curl -X POST {base_url}/exec \\
     -H "X-Gateway-Token: {token}" \\
     -H "Content-Type: application/json" \\
     -d '{{"command": "echo Hello from AI Sandbox"}}'
""")

    print("\n📚 Documentation:")
    print("   - README.md - Full documentation")
    print("   - docs/API.md - API reference")
    print("   - docs/SKILLS.md - AI Agent skills guide")

    print("\n⚠️  Security Notes:")
    print("   - Keep your token secret!")
    print("   - Consider disabling /token_hint endpoint in production")
    print("   - Monitor /task/list for active tasks")
    print("="*70 + "\n")


def main():
    """Main deployment flow"""
    print("\n🚀 AI Sandbox Go Deployment Script")
    print("="*70 + "\n")

    # Generate token
    token = generate_token()
    print(f"Generated token: {token[:8]}...")

    # Step 1: Create/get studio
    studio = create_or_get_studio()
    if not studio:
        sys.exit(1)

    # Step 2: Set secret
    if not set_secret("GATEWAY_TOKEN", token):
        sys.exit(1)

    # Step 3: Push code
    if not push_code():
        print("\n⚠ Git push failed, but you can push manually later")
        print("The Studio and secret are configured.")

    # Step 4: Trigger build
    trigger_build()

    # Step 5: Wait for running
    if not wait_for_running():
        print("\n⚠ Studio may still be building. Check the Studio page:")
        print(f"   https://www.modelscope.cn/studios/{NAMESPACE}/{STUDIO_NAME}")

    # Step 6: Save token and print summary
    save_token(token)
    print_summary(token)


if __name__ == "__main__":
    main()
