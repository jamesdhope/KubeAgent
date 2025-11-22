# Agent Platform

This repository contains a Kubernetes-native platform for orchestrating multi-agent flows using custom resources and a Go-based operator/controller. The platform is designed to support complex agent workflows, LLM integrations, and MCP (Model Context Protocol) servers, with a Python server for agent execution.

## Overview
- **Custom Resource Definition (CRD):** Defines `MultiAgentFlow` resources, allowing users to declaratively specify agent workflows, LLMs, MCP servers, logging, and connections between agents.
- **Go Operator/Controller:** Watches for `MultiAgentFlow` CRs, validates and processes their configuration, and manages the deployment of a Python server for agent execution.
- **Python Server:** Receives agent flow configuration from the controller and executes the defined workflow.

## What the Controller Does
The Go-based controller/operator:
- Watches for new or updated `MultiAgentFlow` custom resources in the cluster.
- Validates the configuration and builds a dependency graph for agents.
- Ensures supporting services (like Redis) and deployments (like the Python server) exist.
- Creates agent pods and writes their dependencies to Redis.
- Monitors agent pod status and updates the CR status when all agents complete.
- Cleans up agent pods after completion.
- Passes the agent workflow configuration to the Python server for execution.

## What the MultiAgentFlow CR Allows Users to Define
The `MultiAgentFlow` custom resource enables users to describe:

- **MCP Servers:**
  - List of MCP servers with name, type, endpoint, and authentication secret references.
- **LLMs (Large Language Models):**
  - List of LLMs with provider, model, parameters, and API key secret references.
- **Logging:**
  - Logging configuration, including integration with Langfuse and secret references for API keys.
- **Agents:**
  - Multiple agents, each with their own LLMs, validation rules, nodes, edges, and MCP server references.
  - Nodes define agent steps, prompts, RAG sources, and queries.
  - Edges define connections between nodes.
- **Connections:**
  - Global connections between agents or nodes, specifying data flow or dependencies.

## Example Use Cases
- Orchestrate multi-agent workflows for LLM-powered applications.
- Integrate multiple LLM providers and models in a single workflow.
- Define complex agent graphs with validation, RAG, and custom prompts.
- Securely manage API keys and secrets for LLMs and MCP servers.

## Getting Started
1. Install the CRD (`multiagentflow-crd.yaml`) in your Kubernetes cluster.
2. Deploy the Go operator/controller.
3. Apply a `MultiAgentFlow` CR to define your agent workflow.
4. The controller will deploy the Python server and pass the configuration for execution.

## Repository Structure
- `crd/`: CRD YAML definitions
- `cr/`: Sample custom resources
- `operator/`: Go controller/operator code
- `python-server/`: Python server for agent execution

## License
MIT
# KubeAgent
