# Windows Updater Architecture & Release Guide

To solve Windows file-locking issues, anti-virus alerts, and package size overhead, the auto-update pipeline uses a dedicated, lightweight **console-based updater** separate from the main application.

---

## 1. How the Updater Works (Architecture)

The update flow is split into a **two-step bootstrap process** to cleanly replace locked files:

```mermaid
sequenceDiagram
    participant App as Main App (vpn-share-tool)
    participant Server as Discovery Server
    participant Updater as Updater (updater.exe)

    App->>Server: 1. Check updates (/latest-app-version)
    Server-->>App: New version available (e.g., v40b)
    App->>Server: 2. Download updater (/latest-version)
    Server-->>App: Returns updater.exe
    App->>Updater: 3. Launch updater.exe & self-terminate
    Note over Updater: 4. Wait for main app to exit
    Updater->>Server: 5. Download app ZIP (/latest-app-version?format=zip)
    Server-->>Updater: Returns vpn-share-tool-app_v40b.zip
    Updater->>Updater: 6. Unzip & overwrite vpn-share-tool.exe
    Updater->>App: 7. Launch updated vpn-share-tool.exe
    Note over Updater: 8. Exit
```

### Modes of Operation
1. **Bootstrap Mode**: When the updater is executed with the name `vpn-share-tool.exe` (such as when legacy clients directly replace their main executable with the updater file), it copies itself to `updater.exe`, spawns `updater.exe --source vpn-share-tool.exe`, and exits immediately.
2. **Upgrade Mode**: Running as `updater.exe` with a `--source` flag, it waits for the main application process to exit and release file locks, downloads the zip release of the actual application, extracts/overwrites the target binary, and restarts the application.

---

## 2. Updater vs. Main App Differences

| Feature | Main Application (`vpn-share-tool`) | Updater (`updater.exe`) |
| :--- | :--- | :--- |
| **Role** | GUI proxy sharing & client dashboard | Lightweight binary replacement agent |
| **Interface**| Fyne Desktop GUI (Interactive) | Console (CLI outputs, progress bars) |
| **Size** | ~64 MB | ~6.5 MB |
| **Go Version**| `go >= 1.25` (Required by Sentry) | `go 1.24` / pure Go |
| **CGO** | Enabled (OpenGL / graphics drivers) | Disabled (`CGO_ENABLED=0`) |
| **Release File**| `vpn-share-tool-app_vXX.zip` | `vpn-share-tool_vXX.exe`, `vpn-share-tool_vXX.zip` |

---

## 3. Build & Release Commands

All compilation and publishing tasks are managed by the custom development CLI (`go run ./dev`).

### A. The Updater Binary
To build and release updates to the updater binary itself (e.g., when improving the updater's unzip logic):

1. **Build**:
   ```bash
   go run ./dev build updater
   ```
   This reads the current version from `Release.toml` and compiles `dist/updater.exe` (~6.5MB) using compile-time optimization and version injection flags (`-trimpath` and `-ldflags="-s -w -X main.UpdaterVersion=vXX"`). This ensures the updater knows its own version and avoids infinite update loops.

2. **Release**:
   ```bash
   go run ./dev release-windows --updater
   ```
   Copies the updater executable and its ZIP package (renamed to `vpn-share-tool_vXX.exe` and `vpn-share-tool_vXX.zip`) to the Samba share path. **No hash (`.sha256`) files are uploaded** to maintain backwards compatibility with legacy client versions (e.g., `37b`).

### B. The Main Application
To build and release updates to the main application:

1. **Build**:
   * **Local build** (using local mingw-w64 toolchain):
     ```bash
     go run ./dev build windows --local
     ```
   * **Container build** (using `fyne-cross` Docker image):
     ```bash
     go run ./dev build windows
     ```
   This bumps the version count, embeds frontend assets, and generates the application executable.

2. **Release**:
   ```bash
   go run ./dev release-windows
   ```
   Compresses the application into `vpn-share-tool-app_vXX.zip` and uploads it to the Samba share alongside its `.sha256` integrity verification file. **Only the `.zip` archive is uploaded for the app** to optimize network traffic and disk usage on the share.

---

## 4. Client Transition & Backwards Compatibility Guide

With the removal of the separate `updater.exe` from the new client updating flow, we transition clients in two steps to maintain compatibility with all client eras (Extremely Old e.g. `v37b`, Medium-Old e.g. `v38` - `v40c`, and New client versions):

### Era 1: Extremely Old Clients (e.g., `v37b`)
- **Behavior**: These clients check `/latest-version?format=zip` directly for the main application update.
- **Transition**: When we serve `vpn-share-tool_vXX.zip` (containing the updater compiled as `vpn-share-tool.exe`), these clients download it and overwrite themselves. On startup, the updater detects it is named `vpn-share-tool.exe` and enters **Bootstrap Mode**, copying itself to `updater.exe`, spawning it, and exiting. The spawned `updater.exe` then downloads and installs `vpn-share-tool-app_vXX.zip` (the new app).

### Era 2: Medium-Old Clients (e.g., `v38` - `v40c`)
- **Behavior**: These clients query `/latest-app-version` for the app update, but download `updater.exe` from `/latest-version` to apply it.
- **Transition Step 1 (Upgrade Clients)**: Publish the new client version using the `vpn-share-tool-app_vXX.zip` naming pattern (e.g., `vpn-share-tool-app_v40d.zip`). Medium-old clients' `updater.exe` will download, extract, and successfully upgrade them to `v40d`. Once upgraded, **no active clients use `updater.exe` anymore**; they all update directly via `update.bat`.
- **Transition Step 2 (Stop Using `-app` Suffix)**: Once all clients run `v40d` or newer, subsequent versions (e.g., `v41a`) can be published directly as the base package `vpn-share-tool_v41a.zip`. The discovery server will fall back to match both `-app` and base package patterns for `/latest-app-version`, updating the clients successfully.
