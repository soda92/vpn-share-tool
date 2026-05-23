# Introduction

The **VPN Share Tool** is a cross-platform distributed utility designed to safely and easily share remote or local network resources across multiple devices on the same local network. 

## The Problem
When working in teams, developers often connect to corporate VPNs or staging networks to access internal testing sites, staging environments, databases, or APIs. However:
1. Connecting multiple devices (e.g., test mobile devices, tablets, additional testing laptops) to the same corporate VPN simultaneously can be restricted, expensive, or complex.
2. Sharing a temporary resource hosted on a developer's localhost with another local testing machine is tedious.

## The Solution
The VPN Share Tool solves this by allowing one machine—which has access to the VPN or local resource—to serve as a localized proxy. 
* It exposes the resource securely over the local network using HTTP/HTTPS.
* Other devices on the same network can access the shared resource without needing to run the VPN client themselves.
* It automatically handles URL rewrites and captures debug traffic.

## Key Features
* **Zero Configuration Discovery**: The client application automatically discovers the registry server on the local subnet without manual IP input.
* **HTTPS and SSL/TLS Support**: Connections to discovery and shared proxies are secured with custom certificate authority (CA) certificates.
* **Embedded Request Debugger**: Built-in web dashboard at `/debug/` to view and inspect all proxied HTTP requests and responses in real-time.
* **Anti-Phishing & Content Rewriter**: Rewrites hyperlinks, absolute assets, redirects, and injects customized client-side helpers (like CAPTCHA solving hooks) so proxied resources render correctly.
* **Auto-Reconnecting Live Logs**: Real-time traffic log streams to a centralized admin registry.
