# Dashboard

```
 █████   ███   █████          █████              
░░███   ░███  ░░███          ░░███               
 ░███   ░███   ░███   ██████  ░███████           
 ░███   ░███   ░███  ███░░███ ░███░░███          
 ░░███  █████  ███  ░███████  ░███ ░███          
  ░░░█████░█████░   ░███░░░   ░███ ░███          
    ░░███ ░░███     ░░██████  ████████           
     ░░░   ░░░       ░░░░░░  ░░░░░░░░            
                                                 
                                                 
                                                 
 ███████████                                ████ 
░░███░░░░░███                              ░░███ 
 ░███    ░███  ██████   ████████    ██████  ░███ 
 ░██████████  ░░░░░███ ░░███░░███  ███░░███ ░███ 
 ░███░░░░░░    ███████  ░███ ░███ ░███████  ░███ 
 ░███         ███░░███  ░███ ░███ ░███░░░   ░███ 
 █████       ░░████████ ████ █████░░██████  █████
░░░░░         ░░░░░░░░ ░░░░ ░░░░░  ░░░░░░  ░░░░░ 
```

This project is built with **Go v1.25.4**.

## 🐳 Building the Container

Build and deploy the web panel as a container using Podman for easy portability across systems.

### 📋 Prerequisites
- **Podman** installed on your build machine (Linux recommended).
- **For Windows**: Podman Desktop or Podman CLI with WSL2 enabled.
- **Note**: The container now includes MySQL and Redis for a self-contained setup. No external DBs needed!

### 🏗️ Build the Image (on Linux)
```bash
podman build -f ContainerFile -t web_panel .
```
This creates a container image with your Go app, dependencies, and tools.

### 💾 Save the Image for Transfer
```bash
podman save web_panel > web_panel.tar
```
Generates a portable tar archive (~2000MB+).

### 🚀 Transfer and Load on Windows
1. **Copy** `web_panel.tar` to your Windows machine.
2. **Load in Podman Desktop**:
   - Open Podman Desktop → **Images** tab → **Import Image**.
   - Select `web_panel.tar` and import.
3. **Or via CLI**:
   ```bash
   podman load < web_panel.tar
   ```

### ▶️ Run the Container
```bash
podman run -p 2221:2221 web_panel
```

|    Feature    |                                 Details                                  |
|---------------|--------------------------------------------------------------------------|
| **Access**    | Open `http://localhost:2221` in your browser                             |
| **Databases** | MySQL (user: `swi`, pwd: `Takasitau`) and Redis run inside the container |
| **Port**      | 2221 (configurable via `config/conf.yaml` inside the container)          |
| **Stop**      | `podman stop <container_id>`                                             |

> **Tip**: Data persists in the container; for production, mount volumes: `podman run -v /host/mysql:/var/lib/mysql -v /host/redis:/var/lib/redis -p 2221:2221 web_panel`.

## Pre-commit Hook

A pre-commit hook has been set up to ensure you're not committing with development configuration. Before each commit, the hook will:

1. Check if `config/conf.yaml` exists
2. Verify that `CONFIG_MODE` is set to `"prod"`
3. If not in production mode, display a warning and ask for confirmation

### To change config to production mode:

**Option 1: Manual edit**
1. Edit `config/conf.yaml`
2. Change `CONFIG_MODE: "dev"` to `CONFIG_MODE: "prod"`

**Option 2: Use the helper script**
```bash
./switch-config.sh
```
Then select option 2 for production mode.

### Bypassing the hook:

If you need to commit with dev config (for testing purposes), the hook will ask for confirmation. Type `y` to proceed or any other key to abort the commit.