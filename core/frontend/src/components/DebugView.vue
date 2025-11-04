<template>
  <div id="app" @click="hideContextMenu">
    <div class="main-layout">
      <div class="request-list-pane">
        <div class="request-list-header">
          <h2>Requests</h2>
          <div class="filter-controls">
            <input type="text" v-model="searchQuery" placeholder="Search URL..." />
            <select v-model="methodFilter">
              <option value="ALL">All</option>
              <option value="GET">GET</option>
              <option value="POST">POST</option>
            </select>
            <button @click="clearHistory">Clear</button>
          </div>
        </div>
        <div v-if="groupedAndFilteredRequests.length === 0" class="no-requests">
          No requests match the filter.
        </div>
        <ul v-else class="request-list">
          <li
            v-for="item in groupedAndFilteredRequests"
            :key="item.id"
            :class="item.type === 'request' ? { selected: selectedRequest && selectedRequest.id === item.request.id } : 'group-header'"
            @click="item.type === 'request' && selectRequest(item.request)"
            @contextmenu.prevent="item.type === 'request' && showContextMenu($event, item.request)"
          >
            <template v-if="item.type === 'request'">
              <span class="timestamp">{{ new Date(item.request.timestamp).toLocaleTimeString() }}</span>
              <span class="method">{{ item.request.method }}</span>
              <span class="url">{{ item.request.url.substring(item.groupName.length) }}</span>
            </template>
            <template v-else>
              <span>{{ item.groupName }}</span>
            </template>
          </li>
        </ul>
        <div
          v-if="contextMenu.visible"
          class="context-menu"
          :style="{ top: contextMenu.y + 'px', left: contextMenu.x + 'px' }"
        >
          <ul>
            <li @click="selectForCompare">Select for Compare</li>
            <li @click="compareWithSelected" :class="{ disabled: !selectedForCompare }">Compare with Selected</li>
            <li @click="shareRequest">Share Request</li>
          </ul>
        </div>
      </div>
      <div class="request-details-pane">
        <div v-if="selectedRequest">
          <h2>Request Details</h2>
          <div class="details-grid">
            <div><strong>URL:</strong></div>
            <div>{{ selectedRequest.url }}</div>
            <div><strong>Method:</strong></div>
            <div>{{ selectedRequest.method }}</div>
            <div><strong>Status:</strong></div>
            <div>{{ selectedRequest.response_status }}</div>
            <div><strong>Timestamp:</strong></div>
            <div>{{ new Date(selectedRequest.timestamp).toLocaleString() }}</div>
          </div>

          <h3>Request Headers</h3>
          <pre>{{ selectedRequest.request_headers }}</pre>

          <h3>Request Body</h3>
          <UrlDecoder
            v-if="isWwwFormUrlEncoded"
            :encodedData="selectedRequest.request_body"
          />
          <pre v-else>{{ selectedRequest.request_body }}</pre>

          <h3>Response Headers</h3>
          <pre>{{ selectedRequest.response_headers }}</pre>

          <h3>Response Body</h3>
          <pre v-if="isJsonResponse">{{ formattedResponseBody }}</pre>
          <pre v-else>{{ selectedRequest.response_body }}</pre>
        </div>
        <div v-else class="no-selection">
          Select a request to see details.
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, defineProps } from 'vue';
import axios from 'axios';
import UrlDecoder from './UrlDecoder.vue';
import { useRouter } from 'vue-router';

const props = defineProps<{
  isLive: boolean;
  sessionId?: string;
}>();

const router = useRouter();

interface CapturedRequest {
  id: number;
  timestamp: string;
  method: string;
  url: string;
  request_headers: Record<string, string[]>;
  request_body: string;
  response_status: number;
  response_headers: Record<string, string[]>;
  response_body: string;
}

const requests = ref<CapturedRequest[]>([]);
const selectedRequest = ref<CapturedRequest | null>(null);
const searchQuery = ref('');
const methodFilter = ref('ALL');
const selectedForCompare = ref<CapturedRequest | null>(null);
const contextMenu = ref({
  visible: false,
  x: 0,
  y: 0,
  request: null as CapturedRequest | null,
});

const filteredRequests = computed(() => {
  return requests.value.filter(req => {
    const methodMatch = methodFilter.value === 'ALL' || req.method === methodFilter.value;
    const searchMatch = req.url.toLowerCase().includes(searchQuery.value.toLowerCase());
    return methodMatch && searchMatch;
  });
});

const getUrlPrefix = (url: string) => {
  try {
    const urlObj = new URL(url);
    const pathParts = urlObj.pathname.split('/').filter(p => p);
    if (pathParts.length > 1) {
      return `${urlObj.origin}/${pathParts.slice(0, -1).join('/')}/`;
    }
    return urlObj.origin + '/';
  } catch (e) {
    const parts = url.split('/');
    if (parts.length > 3) {
      return parts.slice(0, parts.length - 1).join('/') + '/';
    }
    return url;
  }
};

const groupedAndFilteredRequests = computed(() => {
  const result: any[] = [];
  let lastPrefix = '';

  for (const request of filteredRequests.value) {
    const currentPrefix = getUrlPrefix(request.url);
    if (currentPrefix !== lastPrefix) {
      result.push({ id: `group-${currentPrefix}-${request.id}`, type: 'group-header', groupName: currentPrefix });
      lastPrefix = currentPrefix;
    }
    result.push({ id: request.id, type: 'request', request: request, groupName: currentPrefix });
  }

  return result;
});

const isWwwFormUrlEncoded = computed(() => {
  if (!selectedRequest.value) return false;
  const contentType = selectedRequest.value.request_headers['Content-Type']?.[0] || '';
  return contentType.includes('application/x-www-form-urlencoded');
});

const isJsonResponse = computed(() => {
  if (!selectedRequest.value) return false;
  const contentType = selectedRequest.value.response_headers['Content-Type']?.[0] || '';
  return contentType.includes('application/json');
});

const formattedResponseBody = computed(() => {
  if (selectedRequest.value && isJsonResponse.value) {
    try {
      const jsonObj = JSON.parse(selectedRequest.value.response_body);
      return JSON.stringify(jsonObj, null, 2);
    } catch {
      return selectedRequest.value.response_body;
    }
  }
  return selectedRequest.value?.response_body;
});

const fetchRequests = async () => {
  try {
    let url = '';
    if (props.isLive) {
      url = '/debug/live-requests'; // New endpoint for live requests
    } else if (props.sessionId) {
      url = `/debug/sessions/${props.sessionId}/requests`;
    }
    const response = await axios.get(url);
    requests.value = (response.data || []).sort((a: CapturedRequest, b: CapturedRequest) => b.id - a.id);
  } catch (error) {
    console.error('Error fetching requests:', error);
  }
};

const clearHistory = async () => {
  if (props.isLive) {
    try {
      await axios.post('/debug/clear');
      requests.value = [];
      selectedRequest.value = null;
      selectedForCompare.value = null;
    } catch (error) {
      console.error('Error clearing history:', error);
    }
  } else {
    // For saved sessions, clearing history is not applicable
    alert('Cannot clear history for a saved session.');
  }
};

const selectRequest = (request: CapturedRequest) => {
  selectedRequest.value = request;
};

const showContextMenu = (event: MouseEvent, request: CapturedRequest) => {
  contextMenu.value.visible = true;
  contextMenu.value.x = event.clientX;
  contextMenu.value.y = event.clientY;
  contextMenu.value.request = request;
};

const hideContextMenu = () => {
  contextMenu.value.visible = false;
};

const selectForCompare = () => {
  if (contextMenu.value.request) {
    selectedForCompare.value = contextMenu.value.request;
  }
  hideContextMenu();
};

const compareWithSelected = () => {
  if (selectedForCompare.value && contextMenu.value.request) {
    const req1 = selectedForCompare.value;
    const req2 = contextMenu.value.request;

    const req1ContentType = req1.request_headers['Content-Type']?.[0] || '';
    const req2ContentType = req2.request_headers['Content-Type']?.[0] || '';

    const isReq1Json = req1ContentType.includes('application/json');
    const isReq2Json = req2ContentType.includes('application/json');
    const isReq1Form = req1ContentType.includes('application/x-www-form-urlencoded');
    const isReq2Form = req2ContentType.includes('application/x-www-form-urlencoded');

    if ((isReq1Json && isReq2Json) || (isReq1Form && isReq2Form)) {
        window.open(`/debug/compare?req1=${req1.id}&req2=${req2.id}`, '_blank');
    } else {
        alert("Cannot compare requests with different content types (JSON vs. x-www-form-urlencoded).");
    }
  }
  hideContextMenu();
};

const shareRequest = async () => {
  if (!contextMenu.value.request) return;

  let requestIdToShare = contextMenu.value.request.id;

  // If it's a live request, save it first to make it persistent
  if (props.isLive) {
    try {
      const response = await axios.post('/debug/share-request', contextMenu.value.request);
      requestIdToShare = response.data.id;
      alert('Request saved and link copied to clipboard!');
    } catch (error) {
      console.error('Error saving request for sharing:', error);
      alert('Failed to save request for sharing.');
      return;
    }
  }

  const shareUrl = `${window.location.origin}/debug/request/${requestIdToShare}`;
  navigator.clipboard.writeText(shareUrl);
  alert('Share URL copied to clipboard!');
  hideContextMenu();
};

onMounted(() => {
  if (props.isLive) {
    fetchRequests(); // Initial fetch for live view
    const ws = new WebSocket(`ws://${window.location.host}/debug/ws`);

    ws.onmessage = (event) => {
      const newRequest: CapturedRequest = JSON.parse(event.data);
      requests.value.unshift(newRequest); // Add new request to the beginning
      if (requests.value.length > 1000) {
        requests.value.pop(); // Keep only the latest 1000 requests
      }
    };

    ws.onclose = () => {
      console.log('WebSocket connection closed');
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  } else {
    fetchRequests(); // Fetch requests for saved session
  }
  document.title = props.isLive ? "Live Session" : `Session ${props.sessionId}`;
});
</script>

<style scoped>
#app {
  font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
  height: 100vh;
  margin: 0;
  background-color: #f5f5f5;
  color: #333;
}

.main-layout {
  display: flex;
  height: 100%;
}

.request-list-pane {
  width: 35%;
  min-width: 300px;
  border-right: 1px solid #ddd;
  display: flex;
  flex-direction: column;
  background-color: #fff;
}

.request-list-header {
  padding: 1rem;
  border-bottom: 1px solid #ddd;
}

.request-list-header h2 {
  margin: 0 0 1rem 0;
  font-size: 1.25rem;
}

.filter-controls {
  display: flex;
  gap: 0.5rem;
}

.filter-controls input,
.filter-controls select,
.filter-controls button {
  padding: 0.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 0.9rem;
}

.filter-controls input {
  flex-grow: 1;
}

.filter-controls button {
  cursor: pointer;
  background-color: #e0e0e0;
  transition: background-color 0.2s;
}

.filter-controls button:hover {
  background-color: #d0d0d0;
}

.request-list {
  list-style: none;
  padding: 0;
  margin: 0;
  overflow-y: auto;
  flex-grow: 1;
}

.request-list li {
  padding: 0.75rem 1rem;
  cursor: pointer;
  border-bottom: 1px solid #eee;
  display: flex;
  align-items: center;
  gap: 0.75rem;
  transition: background-color 0.2s;
}

.request-list li:hover {
  background-color: #f0f0f0;
}

.request-list li.selected {
  background-color: #d5e5f5;
  border-left: 4px solid #007bff;
  padding-left: calc(1rem - 4px);
}

.method {
  font-weight: 600;
  padding: 0.2rem 0.4rem;
  border-radius: 4px;
  background-color: #e0e0e0;
  font-size: 0.8rem;
  min-width: 40px;
  text-align: center;
}

.url {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.no-requests {
  padding: 1rem;
  text-align: center;
  color: #777;
}

.request-details-pane {
  width: 65%;
  padding: 1.5rem;
  overflow-y: auto;
}

.details-grid {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 0.75rem;
  margin-bottom: 1.5rem;
  background-color: #fff;
  padding: 1rem;
  border-radius: 6px;
  border: 1px solid #ddd;
}

.details-grid div {
  word-break: break-all;
}

h2, h3 {
  border-bottom: 2px solid #007bff;
  padding-bottom: 0.5rem;
  margin-bottom: 1rem;
  color: #0056b3;
}

pre {
  background-color: #fff;
  padding: 1rem;
  border: 1px solid #ddd;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Fira Code', 'Courier New', Courier, monospace;
}

.no-selection {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  font-size: 1.2rem;
  color: #777;
}

.context-menu {
  position: absolute;
  background-color: white;
  border: 1px solid #ccc;
  box-shadow: 2px 2px 5px rgba(0, 0, 0, 0.15);
  border-radius: 4px;
  padding: 0.5rem 0;
  margin: 0;
  list-style: none;
  z-index: 1000;
}

.context-menu ul {
  list-style: none;
  padding: 0;
  margin: 0;
}

.context-menu li {
  padding: 0.6rem 1.2rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.context-menu li:hover {
  background-color: #f0f0f0;
}

.context-menu li.disabled {
  color: #aaa;
  cursor: not-allowed;
  background-color: #fff;
}

.request-list li.group-header {
  background-color: #f8f9fa;
  color: #6c757d;
  font-weight: bold;
  padding: 0.5rem 1rem;
  position: sticky;
  top: 0;
  z-index: 10;
  cursor: default;
  white-space: normal; /* Allow multiline */
  word-break: break-all; /* Break long words */
}

.timestamp {
  font-family: monospace;
  font-size: 0.8rem;
  color: #6c757d;
  min-width: 80px;
}
</style>