# n8n Workflows

This directory is used to store n8n workflow definitions (JSON files).
It is mounted to `/home/node/workflows` inside the n8n container.

## How to add a new workflow

1. Export your workflow from n8n as a JSON file.
2. Save the JSON file in this directory (e.g., `my-workflow.json`).

## How to import workflows

To import a workflow into the running n8n instance, run the following command:

```bash
podman exec -u node -it n8n n8n import:workflow --input=/home/node/workflows/my-workflow.json
```

Or to import all workflows in this directory:

```bash
podman exec -u node -it n8n n8n import:workflow --input=/home/node/workflows
```

## Example

An example workflow `example_flow.json` is provided in this directory.
