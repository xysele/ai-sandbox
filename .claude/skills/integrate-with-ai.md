---
skill: integrate-with-ai
description: Integrate AI Sandbox Gateway with AI agents and LLM applications
---

# Integrate with AI Skill

Use this skill to integrate the AI Sandbox Gateway with AI agents, LLM applications, and automation workflows.

## Quick Start Integration

### Python Integration

```python
import requests
import json

class SandboxClient:
    def __init__(self, base_url="http://localhost:7860", token=None):
        self.base_url = base_url
        self.token = token or os.environ.get("GATEWAY_TOKEN")
        self.headers = {
            "X-Gateway-Token": self.token,
            "Content-Type": "application/json"
        }
    
    def exec(self, command, background=False):
        """Execute a shell command"""
        response = requests.post(
            f"{self.base_url}/exec",
            headers=self.headers,
            json={"command": command, "background": background}
        )
        return response.json()
    
    def batch(self, operations, stop_on_error=True):
        """Execute multiple operations in one request"""
        response = requests.post(
            f"{self.base_url}/batch",
            headers=self.headers,
            json={
                "operations": operations,
                "stop_on_error": stop_on_error
            }
        )
        return response.json()
    
    def read_file(self, path):
        """Read a file"""
        response = requests.post(
            f"{self.base_url}/read_file",
            headers=self.headers,
            json={"path": path}
        )
        result = response.json()
        return result.get("content")
    
    def write_file(self, path, content, base64=False):
        """Write a file"""
        response = requests.post(
            f"{self.base_url}/write_file",
            headers=self.headers,
            json={
                "path": path,
                "content": content,
                "base64": base64
            }
        )
        return response.json()
    
    def write_files(self, files):
        """Write multiple files atomically"""
        response = requests.post(
            f"{self.base_url}/write_files",
            headers=self.headers,
            json={"files": files}
        )
        return response.json()
    
    def create_task(self, command):
        """Create async task"""
        response = requests.post(
            f"{self.base_url}/task/create",
            headers=self.headers,
            json={"command": command}
        )
        return response.json().get("task_id")
    
    def get_task_status(self, task_id):
        """Get task status"""
        response = requests.get(
            f"{self.base_url}/task/status",
            headers=self.headers,
            params={"task_id": task_id}
        )
        return response.json()
    
    def screenshot(self, selector=None):
        """Take screenshot"""
        response = requests.post(
            f"{self.base_url}/gui_screenshot",
            headers=self.headers,
            json={"selector": selector} if selector else {}
        )
        return response.json().get("image")  # base64

# Usage example
sandbox = SandboxClient()

# Execute command
result = sandbox.exec("ls -la /tmp")
print(result["stdout"])

# Batch operations
result = sandbox.batch([
    {"type": "write_file", "params": {"path": "/tmp/test.txt", "content": "hello"}},
    {"type": "read_file", "params": {"path": "/tmp/test.txt"}},
    {"type": "exec", "params": {"command": "cat /tmp/test.txt"}}
])
print(result)
```

### JavaScript/Node.js Integration

```javascript
const axios = require('axios');

class SandboxClient {
  constructor(baseUrl = 'http://localhost:7860', token = null) {
    this.baseUrl = baseUrl;
    this.token = token || process.env.GATEWAY_TOKEN;
    this.headers = {
      'X-Gateway-Token': this.token,
      'Content-Type': 'application/json'
    };
  }

  async exec(command, background = false) {
    const response = await axios.post(
      `${this.baseUrl}/exec`,
      { command, background },
      { headers: this.headers }
    );
    return response.data;
  }

  async batch(operations, stopOnError = true) {
    const response = await axios.post(
      `${this.baseUrl}/batch`,
      { operations, stop_on_error: stopOnError },
      { headers: this.headers }
    );
    return response.data;
  }

  async readFile(path) {
    const response = await axios.post(
      `${this.baseUrl}/read_file`,
      { path },
      { headers: this.headers }
    );
    return response.data.content;
  }

  async writeFile(path, content, base64 = false) {
    const response = await axios.post(
      `${this.baseUrl}/write_file`,
      { path, content, base64 },
      { headers: this.headers }
    );
    return response.data;
  }

  async browserNavigate(url) {
    const response = await axios.post(
      `${this.baseUrl}/browser/navigate`,
      { url },
      { headers: this.headers }
    );
    return response.data;
  }

  async browserScreenshot(fullPage = false) {
    const response = await axios.post(
      `${this.baseUrl}/browser/screenshot`,
      { full_page: fullPage },
      { headers: this.headers }
    );
    return response.data.screenshot;  // base64
  }
}

// Usage
const sandbox = new SandboxClient();

(async () => {
  const result = await sandbox.exec('echo "Hello from Node.js"');
  console.log(result.stdout);
})();
```

## LLM Tool Definitions

### Claude/Anthropic API Format

```json
{
  "name": "execute_command",
  "description": "Execute a shell command in the sandbox environment. Returns stdout, stderr, and exit code.",
  "input_schema": {
    "type": "object",
    "properties": {
      "command": {
        "type": "string",
        "description": "The shell command to execute (supports pipes, redirects, &&, etc.)"
      }
    },
    "required": ["command"]
  }
}
```

```json
{
  "name": "write_files",
  "description": "Write multiple files atomically. All files are written or none if any fails.",
  "input_schema": {
    "type": "object",
    "properties": {
      "files": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "path": {"type": "string"},
            "content": {"type": "string"},
            "base64": {"type": "boolean"}
          },
          "required": ["path", "content"]
        }
      }
    },
    "required": ["files"]
  }
}
```

```json
{
  "name": "batch_operations",
  "description": "Execute multiple API operations in a single request to reduce latency.",
  "input_schema": {
    "type": "object",
    "properties": {
      "operations": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "type": {
              "type": "string",
              "enum": ["exec", "read_file", "write_file", "delete_file", "list_files"]
            },
            "params": {"type": "object"}
          },
          "required": ["type", "params"]
        }
      },
      "stop_on_error": {"type": "boolean", "default": true}
    },
    "required": ["operations"]
  }
}
```

### OpenAI Function Format

```json
{
  "type": "function",
  "function": {
    "name": "sandbox_exec",
    "description": "Execute shell commands in isolated sandbox",
    "parameters": {
      "type": "object",
      "properties": {
        "command": {
          "type": "string",
          "description": "Shell command to execute"
        }
      },
      "required": ["command"]
    }
  }
}
```

## Integration Patterns

### Pattern 1: Code Execution Agent

```python
def code_execution_agent(code, language):
    """Execute code in sandbox and return results"""
    sandbox = SandboxClient()
    
    # Write code to temp file
    filename = f"/tmp/code.{language}"
    sandbox.write_file(filename, code)
    
    # Execute based on language
    runners = {
        "py": f"python3 {filename}",
        "js": f"node {filename}",
        "sh": f"bash {filename}",
        "go": f"go run {filename}"
    }
    
    if language not in runners:
        return {"error": f"Unsupported language: {language}"}
    
    result = sandbox.exec(runners[language])
    
    # Cleanup
    sandbox.exec(f"rm {filename}")
    
    return result

# Usage
result = code_execution_agent("""
print("Hello from Python!")
for i in range(3):
    print(f"Count: {i}")
""", "py")

print(result["stdout"])
```

### Pattern 2: File-based Workflow

```python
def process_data_workflow(data):
    """Multi-step data processing workflow"""
    sandbox = SandboxClient()
    
    # Use batch for efficiency
    operations = [
        # Step 1: Write input data
        {
            "type": "write_file",
            "params": {
                "path": "/tmp/input.json",
                "content": json.dumps(data)
            }
        },
        # Step 2: Process with Python
        {
            "type": "exec",
            "params": {
                "command": "python3 -c 'import json; d=json.load(open(\"/tmp/input.json\")); print(sum(d[\"values\"]))' > /tmp/result.txt"
            }
        },
        # Step 3: Read result
        {
            "type": "read_file",
            "params": {"path": "/tmp/result.txt"}
        }
    ]
    
    result = sandbox.batch(operations)
    return result["results"][-1]["data"]["content"]
```

### Pattern 3: Long-running Task with Polling

```python
import time

def long_running_task(command, timeout=300):
    """Execute long-running command with status updates"""
    sandbox = SandboxClient()
    
    # Create async task
    task_id = sandbox.create_task(command)
    print(f"Task created: {task_id}")
    
    # Poll until complete
    start_time = time.time()
    while True:
        if time.time() - start_time > timeout:
            return {"error": "timeout", "task_id": task_id}
        
        status = sandbox.get_task_status(task_id)
        
        if status["status"] == "completed":
            return status["result"]
        elif status["status"] == "failed":
            return {"error": status.get("error")}
        
        print(f"Status: {status['status']}")
        time.sleep(2)

# Usage
result = long_running_task("npm install && npm test")
```

### Pattern 4: Browser Automation

```python
def scrape_webpage(url, selector):
    """Scrape content from webpage"""
    sandbox = SandboxClient()
    
    # Check browser availability
    status = requests.get(
        f"{sandbox.base_url}/browser/status",
        headers=sandbox.headers
    ).json()
    
    if not status.get("available"):
        return {"error": "Browser not available", "hint": status.get("error")}
    
    # Navigate to page
    sandbox.batch([
        {"type": "browser_navigate", "params": {"url": url}},
        {"type": "browser_wait_for", "params": {"selector": selector}},
    ])
    
    # Take screenshot
    screenshot_response = requests.post(
        f"{sandbox.base_url}/browser/screenshot",
        headers=sandbox.headers,
        json={"full_page": True}
    ).json()
    
    # Evaluate JavaScript to extract data
    data_response = requests.post(
        f"{sandbox.base_url}/browser/evaluate",
        headers=sandbox.headers,
        json={
            "script": f"document.querySelector('{selector}').textContent"
        }
    ).json()
    
    return {
        "screenshot": screenshot_response.get("screenshot"),
        "content": data_response.get("result")
    }
```

## LangChain Integration

```python
from langchain.tools import Tool
from langchain.agents import initialize_agent, AgentType
from langchain.llms import OpenAI

# Create tools
sandbox = SandboxClient()

def execute_command(command: str) -> str:
    result = sandbox.exec(command)
    return f"stdout: {result['stdout']}\nstderr: {result['stderr']}"

def read_file(path: str) -> str:
    return sandbox.read_file(path)

def write_file(path_and_content: str) -> str:
    # Parse "path:content" format
    path, content = path_and_content.split(":", 1)
    result = sandbox.write_file(path.strip(), content.strip())
    return f"Written to {path}"

tools = [
    Tool(
        name="Execute Command",
        func=execute_command,
        description="Execute shell command. Input: command string"
    ),
    Tool(
        name="Read File",
        func=read_file,
        description="Read file content. Input: file path"
    ),
    Tool(
        name="Write File",
        func=write_file,
        description="Write file. Input: 'path:content'"
    )
]

# Initialize agent
llm = OpenAI(temperature=0)
agent = initialize_agent(
    tools,
    llm,
    agent=AgentType.ZERO_SHOT_REACT_DESCRIPTION,
    verbose=True
)

# Use agent
agent.run("Create a Python script that prints 'Hello World' and execute it")
```

## Error Handling Best Practices

```python
def robust_sandbox_call(func, *args, **kwargs):
    """Wrapper with retry and error handling"""
    import time
    
    max_retries = 3
    retry_delay = 2
    
    for attempt in range(max_retries):
        try:
            response = func(*args, **kwargs)
            
            if isinstance(response, dict) and not response.get("success"):
                error = response.get("error", "Unknown error")
                
                # Handle specific errors
                if "unauthorized" in error.lower():
                    raise Exception("Authentication failed - check GATEWAY_TOKEN")
                elif "not found" in error.lower():
                    return {"error": "Resource not found", "recoverable": False}
                elif "timeout" in error.lower():
                    if attempt < max_retries - 1:
                        time.sleep(retry_delay)
                        continue
                    raise Exception("Operation timed out after retries")
                
                return response
            
            return response
            
        except requests.exceptions.ConnectionError:
            if attempt < max_retries - 1:
                print(f"Connection failed, retrying in {retry_delay}s...")
                time.sleep(retry_delay)
                continue
            raise Exception("Cannot connect to sandbox")
        
        except Exception as e:
            if attempt < max_retries - 1:
                time.sleep(retry_delay)
                continue
            raise
    
    raise Exception("Max retries exceeded")
```

## Performance Tips

1. **Use batch endpoint** for multiple operations
   ```python
   # Instead of 3 separate requests
   sandbox.write_file("/tmp/a.txt", "a")
   sandbox.write_file("/tmp/b.txt", "b")
   sandbox.read_file("/tmp/a.txt")
   
   # Use batch (1 request)
   sandbox.batch([
       {"type": "write_file", "params": {"path": "/tmp/a.txt", "content": "a"}},
       {"type": "write_file", "params": {"path": "/tmp/b.txt", "content": "b"}},
       {"type": "read_file", "params": {"path": "/tmp/a.txt"}}
   ])
   ```

2. **Reuse HTTP connections**
   ```python
   session = requests.Session()
   session.headers.update({"X-Gateway-Token": token})
   # Use session instead of requests
   ```

3. **Use async tasks** for long operations
   ```python
   # Don't block on long commands
   task_id = sandbox.create_task("apt-get update && apt-get install -y package")
   # Poll or wait
   ```

4. **Handle base64 for binary files**
   ```python
   import base64
   with open("image.png", "rb") as f:
       content = base64.b64encode(f.read()).decode()
   sandbox.write_file("/tmp/image.png", content, base64=True)
   ```

## Security Considerations

1. **Never hardcode tokens**
   ```python
   # BAD
   token = "my-secret-token-123"
   
   # GOOD
   token = os.environ.get("GATEWAY_TOKEN")
   ```

2. **Validate user input** before passing to sandbox
   ```python
   # Sanitize paths
   if ".." in user_path or user_path.startswith("/"):
       raise ValueError("Invalid path")
   ```

3. **Use HTTPS** in production
   ```python
   sandbox = SandboxClient(base_url="https://sandbox.example.com")
   ```

4. **Limit command capabilities** based on use case
   ```python
   ALLOWED_COMMANDS = ["ls", "cat", "echo", "grep"]
   if not any(cmd in user_command for cmd in ALLOWED_COMMANDS):
       raise ValueError("Command not allowed")
   ```
