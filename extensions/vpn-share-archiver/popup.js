let activeAPI = "http://127.0.0.1:10081";

document.addEventListener("DOMContentLoaded", async () => {
  const statusBadge = document.getElementById("status-badge");
  const proxyInfoSection = document.getElementById("proxy-info-section");
  const proxyOrigUrl = document.getElementById("proxy-orig-url");
  const proxyLocalPort = document.getElementById("proxy-local-port");

  const sessionNameInput = document.getElementById("session-name-input");
  const startRecBtn = document.getElementById("start-rec-btn");
  const stopRecBtn = document.getElementById("stop-rec-btn");
  const startRecGroup = document.getElementById("start-recording-group");
  const stopRecGroup = document.getElementById("stop-recording-group");
  const activeSessionId = document.getElementById("active-session-id");

  const historyContainer = document.getElementById("history-container");
  const openDashboardBtn = document.getElementById("open-dashboard-btn");

  let activeTab = null;
  let activeProxy = null;
  let pageOriginalURL = "";

  // 1. Get current active tab
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    activeTab = tab;
  } catch (err) {
    console.error("Failed to query tabs:", err);
  }

  // Helper to dynamically detect remote API host using injected meta tags or hostname fallback
  async function detectAPIEndpoint(tab) {
    if (!tab || !tab.url) return null;

    try {
      // Execute content script to look for vpn-share-api meta tags
      const results = await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: () => {
          const apiMeta = document.querySelector('meta[name="vpn-share-api"]');
          const portMeta = document.querySelector('meta[name="vpn-share-proxy-port"]');
          return {
            api: apiMeta ? apiMeta.content : null,
            port: portMeta ? parseInt(portMeta.content, 10) : null
          };
        }
      });

      if (results && results[0] && results[0].result) {
        const { api, port } = results[0].result;
        if (api) {
          return { api, port };
        }
      }
    } catch (err) {
      console.log("Could not inspect page DOM (likely a non-HTML resource or permission limit):", err);
    }

    // Fallback: Use active tab's hostname on port 10081
    try {
      const tabUrl = new URL(tab.url);
      if (tabUrl.port && tabUrl.port !== "10081") {
        return {
          api: `http://${tabUrl.hostname}:10081`,
          port: parseInt(tabUrl.port, 10)
        };
      }
    } catch (err) {
      console.error("Fallback hostname parsing failed:", err);
    }

    return null;
  }

  // 2. Poll API status
  async function checkStatus() {
    if (!activeTab || !activeTab.url) {
      statusBadge.textContent = "Offline";
      statusBadge.className = "badge badge-disconnected";
      return;
    }

    // Detect the API Endpoint dynamically
    const detected = await detectAPIEndpoint(activeTab);
    let tabPort = 0;
    if (detected) {
      activeAPI = detected.api;
      tabPort = detected.port;
    } else {
      const tabUrl = new URL(activeTab.url);
      tabPort = parseInt(tabUrl.port, 10) || 80;
      activeAPI = `http://${tabUrl.hostname}:10081`;
    }

    try {
      const response = await fetch(`${activeAPI}/active-proxies`);
      if (!response.ok) throw new Error("API Offline");
      
      const proxies = await response.json();

      // Match tab's port with any proxy's RemotePort on the target API server
      activeProxy = proxies.find(p => p.remote_port === tabPort);

      if (activeProxy) {
        // Connected!
        statusBadge.textContent = activeProxy.is_recording ? "Recording" : "Connected";
        statusBadge.className = activeProxy.is_recording ? "badge badge-recording" : "badge badge-connected";

        proxyOrigUrl.textContent = activeProxy.original_url;
        proxyLocalPort.textContent = activeProxy.remote_port;
        proxyInfoSection.style.display = "block";

        // Reconstruct original page URL
        const originalProxyURL = new URL(activeProxy.original_url);
        const tabUrl = new URL(activeTab.url);
        pageOriginalURL = originalProxyURL.protocol + "//" + originalProxyURL.host + tabUrl.pathname + tabUrl.search;

        // Toggle groups
        if (activeProxy.is_recording) {
          startRecGroup.style.display = "none";
          stopRecGroup.style.display = "block";
          activeSessionId.textContent = activeProxy.active_session_id.substring(0, 8) + "...";
        } else {
          startRecGroup.style.display = "block";
          stopRecGroup.style.display = "none";
        }

        // Fetch page snapshot history
        await loadHistory();
      } else {
        // Not a proxied page
        statusBadge.textContent = "Unproxied Page";
        statusBadge.className = "badge badge-disconnected";
        proxyInfoSection.style.display = "none";
        startRecGroup.style.display = "block";
        stopRecGroup.style.display = "none";
        disableControls("Open a proxied URL to record");
      }
    } catch (err) {
      statusBadge.textContent = "Offline";
      statusBadge.className = "badge badge-disconnected";
      proxyInfoSection.style.display = "none";
      disableControls(`API Offline (${new URL(activeAPI).host})`);
    }
  }

  function disableControls(placeholderText) {
    sessionNameInput.disabled = true;
    sessionNameInput.placeholder = placeholderText;
    startRecBtn.disabled = true;
    stopRecBtn.disabled = true;
    historyContainer.innerHTML = `<li class="empty-state">${placeholderText}</li>`;
  }

  async function loadHistory() {
    if (!pageOriginalURL) return;

    try {
      const res = await fetch(`${activeAPI}/archive/history?url=${encodeURIComponent(pageOriginalURL)}`);
      if (!res.ok) throw new Error("History error");
      const history = await res.json();

      if (history && history.length > 0) {
        historyContainer.innerHTML = "";
        history.forEach(item => {
          const li = document.createElement("li");
          li.className = "history-item";
          
          const a = document.createElement("a");
          a.className = "history-link";
          a.href = `${activeAPI}${item.playback_url}`;
          a.target = "_blank";
          a.textContent = item.formatted;

          const span = document.createElement("span");
          span.className = "history-meta";
          span.textContent = item.session_name;

          li.appendChild(a);
          li.appendChild(span);
          historyContainer.appendChild(li);
        });
      } else {
        historyContainer.innerHTML = '<li class="empty-state">No snapshots found for this page.</li>';
      }
    } catch (err) {
      historyContainer.innerHTML = '<li class="empty-state">Failed to load snapshot history.</li>';
    }
  }

  // 3. Button click handlers
  startRecBtn.addEventListener("click", async () => {
    if (!activeProxy) return;

    const name = sessionNameInput.value.trim() || `Session ${new Date().toLocaleTimeString()}`;
    startRecBtn.disabled = true;
    startRecBtn.textContent = "Starting...";

    try {
      const res = await fetch(`${activeAPI}/archive/toggle-recording`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          proxy_port: activeProxy.remote_port,
          name: name,
          enable: true
        })
      });

      if (!res.ok) throw new Error("Failed to start");
      sessionNameInput.value = "";
      await checkStatus();
    } catch (err) {
      alert("Error starting recording session: " + err.message);
    } finally {
      startRecBtn.disabled = false;
      startRecBtn.textContent = "Start Recording Session";
    }
  });

  stopRecBtn.addEventListener("click", async () => {
    if (!activeProxy) return;

    stopRecBtn.disabled = true;
    stopRecBtn.textContent = "Stopping...";

    try {
      const res = await fetch(`${activeAPI}/archive/toggle-recording`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          proxy_port: activeProxy.remote_port,
          enable: false
        })
      });

      if (!res.ok) throw new Error("Failed to stop");
      await checkStatus();
    } catch (err) {
      alert("Error stopping recording session: " + err.message);
    } finally {
      stopRecBtn.disabled = false;
      stopRecBtn.textContent = "Stop Recording";
    }
  });

  openDashboardBtn.addEventListener("click", () => {
    chrome.tabs.create({ url: `${activeAPI}/debug/#/archives` });
  });

  // Initial check
  await checkStatus();
});
