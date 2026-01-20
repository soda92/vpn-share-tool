# VPN Share Tool

A tool to share VPN connections over the local network.

## Build Instructions

This project uses a unified development CLI tool `dev` (invoked via `dev.fish`) to handle building, running, and deploying components.

### Prerequisites

- Go 1.21+
- Node.js & npm (for frontend)
- `fyne` (for Android/Desktop builds)
- `fyne-cross` (for Windows cross-compilation)
- `gomobile` (for Android AAR)
- Fish shell (optional, but the wrapper script is `dev.fish`)

### Commands

**Build Discovery Server:**

```fish
./dev.fish build server
```
This will build the frontend (`discovery_web`) and the backend (`cmd/discovery`), placing the binary in `dist/discovery`.

**Build Desktop App (Local):**

```fish
./dev.fish build
```

**Build for Windows (Cross-compile):**

```fish
./dev.fish build windows
```

**Run Desktop App (Dev):**

```fish
./dev.fish run
```

### Manual Build

If you prefer standard Go commands:

1.  **Discovery Server:**
    ```bash
    # Build Frontend
    cd discovery_web
    npm install
    npm run build
    # Move dist to api package for embedding
    rm -rf ../discovery/api/dist
    mv dist ../discovery/api/dist
    cd ..
    
    # Build Backend
    go build -o dist/discovery ./cmd/discovery
    ```

2.  **Desktop App:**
    ```bash
    # Build Frontend
    cd core/debug_web
    npm install
    npm run build
    cd ../..
    
    # Build Backend
    go build -o vpn-share-tool ./cmd/vpn-share-tool
    ```
