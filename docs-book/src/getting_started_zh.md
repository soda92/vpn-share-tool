# 快速开始

本指南将介绍如何设置本地开发环境、构建项目并运行应用程序。

## 前置环境要求

* **Go**：版本 `1.24.0` 或更高。
* **Node.js & npm**：版本 `20` 或更高（用于编译 Web 前端资源）。
* **Rust**：（可选）如果你需要编译或调试 `rustlib` 目录下的 FFI 组件。

---

## 开发环境搭建

1. **克隆仓库**：
   ```bash
   git clone https://github.com/soda92/vpn-share-tool.git
   cd vpn-share-tool
   ```

2. **生成开发证书**：
   在编译之前，你需要生成开发用的自签名 TLS 证书：
   ```bash
   go run ./dev certs
   ```
   这将在 `certs/` 目录下生成 `ca.crt`、`ca.key`、`server.crt` 和 `server.key`。

3. **安装前端依赖**：
   分别在两个 Web 前端项目中安装 npm 依赖包：
   ```bash
   # 请求调试器前端 (Request Debugger)
   cd core/debug_web
   npm install
   cd ../..

   # 注册中心管理台前端 (Discovery Dashboard)
   cd discovery_web
   npm install
   cd ..
   ```

---

## 编译项目

你可以使用内置的命令行开发工具（dev CLI）完成各项编译操作：

* **构建桌面 GUI 应用程序**：
  ```bash
  go run ./dev build
  ```
  这会先编译 Vue 前端资源，然后将其打包嵌入最终的 Go 二进制文件中，输出至 `cmd/vpn-share-tool/vpn-share-tool`。

* **构建注册服务 (Discovery Server)**：
  ```bash
  go run ./dev build server
  ```
  这会编译管理台前端资源并编译服务端的二进制文件，输出至 `dist/discovery`。

* **构建 Windows 安装包 (在 Linux Docker 环境下交叉编译)**：
  ```bash
  go run ./dev build windows
  ```

* **构建 Linux FFI 共享库 (供 Flutter FFI 绑定调用)**：
  ```bash
  go run ./dev build linux-so
  ```

---

## 运行项目

1. **启动注册服务器 (Discovery Server)**：
   ```bash
   # 启动服务端
   ./dist/discovery
   ```
   默认情况下，它将在 TCP `45679` 端口监听客户端握手，并在 `8080` 端口托管 Web 管理后台。

2. **启动桌面客户端**：
   ```bash
   # 使用开发命令行启动桌面 GUI
   go run ./dev run
   ```
   客户端将启动跨平台 GUI 窗口，并自动开始在子网内广播扫描注册服务器建立安全配对连接。
