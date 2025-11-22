# Refactored agent worker for per-agent pod execution with Redis coordination
import os
import json
import time
import redis
import requests

def load_secrets(secret_names):
    return {k: os.environ.get(k) for k in secret_names if k != "langfuse-api-key"}

def wait_for_dependencies(r, graph_id, agent_name, dependencies):
    for dep in dependencies:
        key = f"{graph_id}:{dep}:output"
        while not r.exists(key):
            print(f"Waiting for dependency: {key}")
            time.sleep(1)

def publish_output(r, graph_id, agent_name, output):
    key = f"{graph_id}:{agent_name}:output"
    try:
        result = r.set(key, json.dumps(output))
        print(f"Published output for {agent_name} to Redis: {key}, result: {result}")
    except Exception as e:
        print(f"ERROR publishing output for {agent_name} to Redis: {key}, error: {e}")

def publish_completion(r, graph_id, agent_name):
    complete_key = f"{graph_id}:{agent_name}:complete"
    try:
        result = r.set(complete_key, "true")
        print(f"Marked {agent_name} as complete in Redis: {complete_key}, result: {result}")
    except Exception as e:
        print(f"ERROR marking {agent_name} as complete in Redis: {complete_key}, error: {e}")

def main():
    agent_spec = json.loads(os.environ["AGENT_SPEC"])
    agent_name = os.environ["AGENT_NAME"]
    graph_id = os.environ["GRAPH_ID"]
    redis_host = os.environ.get("REDIS_HOST", "redis")
    redis_port = int(os.environ.get("REDIS_PORT", "6379"))
    r = redis.Redis(host=redis_host, port=redis_port)

    # Load secrets
    secret_names = ["openai-api-key", "anthropic-api-key"]
    secrets = load_secrets(secret_names)

    # MCP support (simulate connection)
    mcps = agent_spec.get("mcp", [])
    for mcp in mcps:
        print(f"Connecting to MCP: {mcp.get('name')} at {mcp.get('endpoint')}")

    # LLM support (simulate)
    llms = agent_spec.get("llms", [])
    llm_provider = llms[0].get("provider", "openai") if llms else "openai"
    llm_model = llms[0].get("model", "gpt-3.5-turbo") if llms else "gpt-3.5-turbo"
    api_key = llms[0].get("apiKeySecretRef", "openai-api-key") if llms else "openai-api-key"
    print(f"Using LLM provider: {llm_provider}, model: {llm_model}, api_key: {secrets.get(api_key)}")

    # Wait for dependencies (from Redis)
    dep_key = f"{graph_id}:dependencies:{agent_name}"
    dep_bytes = r.get(dep_key)
    dependencies = []
    dep_outputs = {}
    if dep_bytes:
        try:
            dependencies = json.loads(dep_bytes)
        except Exception:
            dependencies = []
    for dep in dependencies:
        complete_key = f"{graph_id}:{dep}:complete"
        output_key = f"{graph_id}:{dep}:output"
        while not r.exists(complete_key):
            print(f"Waiting for dependency completion: {complete_key}")
            time.sleep(1)
        # Read dependency output (after completion)
        dep_output_raw = r.get(output_key)
        try:
            dep_outputs[dep] = json.loads(dep_output_raw) if dep_output_raw else None
        except Exception:
            dep_outputs[dep] = dep_output_raw
    print(f"Dependency outputs for {agent_name}: {dep_outputs}")

    # Simulate node/edge graph logic
    nodes = agent_spec.get("nodes", [])
    edges = agent_spec.get("edges", [])
    print(f"Processing nodes: {nodes} and edges: {edges}")
    # If prompt needs dependency outputs, inject them (simple example)
    if nodes and dep_outputs:
        # Replace {dep_outputs} in prompt with actual outputs
        for node in nodes:
            if "prompt" in node and "{dep_outputs}" in node["prompt"]:
                node["prompt"] = node["prompt"].replace("{dep_outputs}", json.dumps(dep_outputs))

    # LLM output logic
    if llm_provider == "ollama":
        ollama_url = "http://host.docker.internal:11434/api/generate"
        prompt = nodes[0].get("prompt", "Hello from Ollama!") if nodes else "Hello from Ollama!"
        response = requests.post(
            ollama_url,
            json={"model": llm_model, "prompt": prompt}
        )
        result = response.json().get("response", "")
        print(f"Ollama result: {result}")
        output = {"result": result, "status": "completed"}
    else:
        output = {"result": f"Processed by {agent_name}", "status": "completed"}

    # Validation
    validations = agent_spec.get("validation", [])
    validation_results = []
    for check in validations:
        rule = check.get("rule", "")
        try:
            local_vars = {"output": output.get("result"), "status": output.get("status"), "result": output.get("result")}
            passed = eval(rule, {}, local_vars)
        except Exception:
            passed = False
        validation_results.append({"rule": rule, "passed": passed})
    output["validation_results"] = validation_results

    # Only publish output if all validations passed
    if all(v["passed"] for v in validation_results):
        publish_output(r, graph_id, agent_name, output)
        # Mark agent as complete explicitly
        publish_completion(r, graph_id, agent_name)
        print(f"Agent {agent_name} completed and published output.")
    else:
        print(f"Agent {agent_name} did not pass validation, output not published.")

if __name__ == "__main__":
    main()
