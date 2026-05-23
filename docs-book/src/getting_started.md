# Getting Started

This guide explains how to set up the development environment, build the projects, and run the applications.

## Prerequisites

* **Go**: version `1.24.0` or higher.
* **Node.js & npm**: version `20` or higher (required to build the web interfaces).
* **Rust**: (optional) if compiling or checking FFI components under `rustlib`.

---

## Development Environment Setup

1. **Clone the Repository**:
   ```bash
   git clone https://github.com/soda92/vpn-share-tool.git
   cd vpn-share-tool
   ```

2. **Generate Development Certificates**:
   Before compiling, you need to generate self-signed TLS certificates for development:
   ```bash
   go run ./dev certs
   ```
   This will create a `certs/` folder with `ca.crt`, `ca.key`, `server.crt`, and `server.key`.

3. **Install Frontend Dependencies**:
   Install npm dependencies for both web dashboards:
   ```bash
   # Request Debugger frontend
   cd core/debug_web
   npm install
   cd ../..

   # Discovery Dashboard frontend
   cd discovery_web
   npm install
   cd ..
   ```

---

## Building the Applications

You can compile all components using the custom development CLI tool:

* **Build the Desktop GUI Application**:
  ```bash
  go run ./dev build
  ```
  This builds the Vue frontend and compiles the final Go binary to `cmd/vpn-share-tool/vpn-share-tool`.

* **Build the Discovery Server**:
  ```bash
  go run ./dev build server
  ```
  This builds the server dashboard frontend and compiles the server binary to `dist/discovery`.

* **Build for Windows (Cross-compilation via Docker/fyne-cross)**:
  ```bash
  go run ./dev build windows
  ```

* **Build Linux FFI Shared Library (for Flutter)**:
  ```bash
  go run ./dev build linux-so
  ```

---

## Running the Applications

1. **Start the Discovery Server**:
   ```bash
   # Run the server binary
   ./dist/discovery
   ```
   By default, it will listen for client connections on TCP port `45679` and host the web dashboard on port `8080`.

2. **Start the Client Application**:
   ```bash
   # Run the desktop app using the dev CLI
   go run ./dev run
   ```
   The client will boot up the cross-platform GUI, search for the local discovery server on your subnet, and establish a secure registration.
