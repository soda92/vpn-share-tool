import React, { useState, useEffect } from 'react';
import ReactDOM from 'react-dom/client';
import './popup.css';

function Popup() {
  const [activeAPI, setActiveAPI] = useState("http://127.0.0.1:10081");
  const [activeTab, setActiveTab] = useState(null);
  const [activeProxy, setActiveProxy] = useState(null);
  const [pageOriginalURL, setPageOriginalURL] = useState("");
  const [sessionName, setSessionName] = useState("");
  const [history, setHistory] = useState([]);
  const [status, setStatus] = useState({ text: "Checking...", className: "badge-disconnected" });
  const [errorMessage, setErrorMessage] = useState("");
  const [isActionPending, setIsActionPending] = useState(false);

  // Helper to dynamically detect remote API host using injected meta tags or hostname fallback
  const detectAPIEndpoint = async (tab) => {
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
  };

  // Get active tab on mount
  useEffect(() => {
    const getTab = async () => {
      try {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        setActiveTab(tab);
      } catch (err) {
        console.error("Failed to query tabs:", err);
      }
    };
    getTab();
  }, []);

  // Poll status & fetch history
  useEffect(() => {
    if (!activeTab) return;

    const checkStatus = async () => {
      if (!activeTab.url) {
        setStatus({ text: "Offline", className: "badge-disconnected" });
        return;
      }

      let detectedAPI = "http://127.0.0.1:10081";
      let tabPort = 0;
      
      const detected = await detectAPIEndpoint(activeTab);
      if (detected) {
        detectedAPI = detected.api;
        tabPort = detected.port;
      } else {
        try {
          const tabUrl = new URL(activeTab.url);
          tabPort = parseInt(tabUrl.port, 10) || 80;
          detectedAPI = `http://${tabUrl.hostname}:10081`;
        } catch (e) {
          console.error("Failed to parse URL:", e);
        }
      }
      setActiveAPI(detectedAPI);

      try {
        const response = await fetch(`${detectedAPI}/active-proxies`);
        if (!response.ok) throw new Error("API Offline");

        const proxies = await response.json();
        const matchedProxy = proxies.find(p => p.remote_port === tabPort);

        if (matchedProxy) {
          setActiveProxy(matchedProxy);
          setStatus({
            text: matchedProxy.is_recording ? "Recording" : "Connected",
            className: matchedProxy.is_recording ? "badge-recording" : "badge-connected"
          });
          setErrorMessage("");

          // Reconstruct original page URL
          const originalProxyURL = new URL(matchedProxy.original_url);
          const tabUrl = new URL(activeTab.url);
          const reconstructedURL = originalProxyURL.protocol + "//" + originalProxyURL.host + tabUrl.pathname + tabUrl.search;
          setPageOriginalURL(reconstructedURL);
        } else {
          setActiveProxy(null);
          setStatus({ text: "Unproxied Page", className: "badge-disconnected" });
          setErrorMessage("Open a proxied URL to record");
          setHistory([]);
          setPageOriginalURL("");
        }
      } catch (err) {
        setActiveProxy(null);
        setStatus({ text: "Offline", className: "badge-disconnected" });
        let host = "API host";
        try {
          host = new URL(detectedAPI).host;
        } catch (e) {}
        setErrorMessage(`API Offline (${host})`);
        setHistory([]);
        setPageOriginalURL("");
      }
    };

    checkStatus();
    const interval = setInterval(checkStatus, 5000);
    return () => clearInterval(interval);
  }, [activeTab]);

  // Fetch history when pageOriginalURL changes
  useEffect(() => {
    if (!pageOriginalURL) return;

    const loadHistory = async () => {
      try {
        const res = await fetch(`${activeAPI}/archive/history?url=${encodeURIComponent(pageOriginalURL)}`);
        if (!res.ok) throw new Error("History error");
        const historyData = await res.json();
        
        if (historyData && historyData.length > 0) {
          // Sort history by timestamp (latest first)
          const sortedHistory = [...historyData].sort((a, b) => {
            if (a.timestamp > b.timestamp) return -1;
            if (a.timestamp < b.timestamp) return 1;
            return 0;
          });
          setHistory(sortedHistory);
        } else {
          setHistory([]);
        }
      } catch (err) {
        console.error("Failed to load history:", err);
      }
    };

    loadHistory();
  }, [pageOriginalURL, activeAPI]);

  const handleStartRecording = async () => {
    if (!activeProxy) return;

    const name = sessionName.trim() || `Session ${new Date().toLocaleTimeString()}`;
    setIsActionPending(true);

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
      setSessionName("");
      
      // Instantly trigger status refresh
      const response = await fetch(`${activeAPI}/active-proxies`);
      if (response.ok) {
        const proxies = await response.json();
        const matchedProxy = proxies.find(p => p.remote_port === activeProxy.remote_port);
        if (matchedProxy) {
          setActiveProxy(matchedProxy);
          setStatus({
            text: matchedProxy.is_recording ? "Recording" : "Connected",
            className: matchedProxy.is_recording ? "badge-recording" : "badge-connected"
          });
        }
      }
    } catch (err) {
      alert("Error starting recording session: " + err.message);
    } finally {
      setIsActionPending(false);
    }
  };

  const handleStopRecording = async () => {
    if (!activeProxy) return;

    setIsActionPending(true);

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
      
      // Instantly trigger status refresh
      const response = await fetch(`${activeAPI}/active-proxies`);
      if (response.ok) {
        const proxies = await response.json();
        const matchedProxy = proxies.find(p => p.remote_port === activeProxy.remote_port);
        if (matchedProxy) {
          setActiveProxy(matchedProxy);
          setStatus({
            text: matchedProxy.is_recording ? "Recording" : "Connected",
            className: matchedProxy.is_recording ? "badge-recording" : "badge-connected"
          });
        }
      }
    } catch (err) {
      alert("Error stopping recording session: " + err.message);
    } finally {
      setIsActionPending(false);
    }
  };

  const handleOpenDashboard = () => {
    chrome.tabs.create({ url: `${activeAPI}/debug/#/archives` });
  };

  return (
    <>
      <div className="header">
        <h2>VPN Share Archiver</h2>
        <span id="status-badge" className={`badge ${status.className}`}>
          {status.text}
        </span>
      </div>

      {activeProxy && (
        <div id="proxy-info-section" className="section">
          <div className="section-title">Proxy Info</div>
          <div className="info-row">
            <span>Original URL:</span>
            <span id="proxy-orig-url" className="info-val">{activeProxy.original_url}</span>
          </div>
          <div className="info-row">
            <span>Local Port:</span>
            <span id="proxy-local-port" className="info-val">{activeProxy.remote_port}</span>
          </div>
        </div>
      )}

      <div className="section">
        <div className="section-title">Recording Controls</div>
        <div className="control-group">
          {(!activeProxy || !activeProxy.is_recording) ? (
            <div id="start-recording-group">
              <input
                type="text"
                id="session-name-input"
                className="input-field"
                placeholder={errorMessage || "Session Name (e.g. Test Flow)"}
                value={sessionName}
                onChange={(e) => setSessionName(e.target.value)}
                disabled={!!errorMessage}
                style={{ marginBottom: '10px' }}
              />
              <button
                id="start-rec-btn"
                className="btn btn-primary"
                onClick={handleStartRecording}
                disabled={!!errorMessage || isActionPending}
              >
                {isActionPending ? "Starting..." : "Start Recording Session"}
              </button>
            </div>
          ) : (
            <div id="stop-recording-group">
              <div className="info-row" style={{ marginBottom: '10px' }}>
                <span>Active Session ID:</span>
                <span id="active-session-id" className="info-val">
                  {activeProxy.active_session_id ? `${activeProxy.active_session_id.substring(0, 8)}...` : '-'}
                </span>
              </div>
              <button
                id="stop-rec-btn"
                className="btn btn-danger"
                onClick={handleStopRecording}
                disabled={isActionPending}
              >
                {isActionPending ? "Stopping..." : "Stop Recording"}
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="section">
        <div className="section-title">History of this page</div>
        <ul id="history-container" className="history-list">
          {errorMessage ? (
            <li className="empty-state">{errorMessage}</li>
          ) : history.length > 0 ? (
            history.map((item, index) => (
              <li key={index} className="history-item">
                <a
                  className="history-link"
                  href={`${activeAPI}${item.playback_url}`}
                  target="_blank"
                  rel="noreferrer"
                >
                  {item.formatted}
                </a>
                <span className="history-meta">{item.session_name}</span>
              </li>
            ))
          ) : (
            <li className="empty-state">No snapshots found for this page.</li>
          )}
        </ul>
      </div>

      <button
        id="open-dashboard-btn"
        className="btn btn-outline"
        onClick={handleOpenDashboard}
        style={{ marginTop: '10px' }}
      >
        Open Debugger Dashboard
      </button>
    </>
  );
}

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(<Popup />);
