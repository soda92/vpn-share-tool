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
        <div v-if="filteredRequests.length === 0" class="no-requests">
          No requests match the filter.
        </div>
        <ul v-else class="request-list">
          <li
            v-for="request in filteredRequests"
            :key="request.id"
            @click="selectRequest(request)"
            @contextmenu.prevent="showContextMenu($event, request)"
            :class="{ selected: selectedRequest && selectedRequest.id === request.id }"
          >
            <span class="method">{{ request.method }}</span>
            <span class="url">{{ request.url }}</span>
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
          <pre>{{ selectedRequest.response_body }}</pre>
        </div>
        <div v-else class="no-selection">
          Select a request to see details.
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import UrlDecoder from './components/UrlDecoder.vue';

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

const isWwwFormUrlEncoded = computed(() => {
  if (!selectedRequest.value) return false;
  const contentType = selectedRequest.value.request_headers['Content-Type']?.[0] || '';
  return contentType.includes('application/x-www-form-urlencoded');
});

const fetchRequests = async () => {
  try {
    const response = await fetch('/debug/requests');
    if (response.ok) {
      requests.value = (await response.json()).sort((a: CapturedRequest, b: CapturedRequest) => b.id - a.id);
    } else {
      console.error('Failed to fetch requests');
    }
  } catch (error) {
    console.error('Error fetching requests:', error);
  }
};

const clearHistory = async () => {
  try {
    const response = await fetch('/debug/clear', { method: 'POST' });
    if (response.ok) {
      requests.value = [];
      selectedRequest.value = null;
      selectedForCompare.value = null;
    } else {
      console.error('Failed to clear history');
    }
  } catch (error) {
    console.error('Error clearing history:', error);
  }
};

const selectRequest = (request: CapturedRequest) => {
  selectedRequest.value = request;
};

const showContextMenu = (event: MouseEvent, request: CapturedRequest) => {
  if (request.method !== 'POST') return;
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

onMounted(() => {
  fetchRequests();
  setInterval(fetchRequests, 2000);
});
</script>

<style scoped>
#app {
  font-family: sans-serif;
  height: 100vh;
  margin: 0;
}

.main-layout {
  display: flex;
  height: 100%;
}

.request-list-pane {
  width: 30%;
  border-right: 1px solid #ccc;
  display: flex;
  flex-direction: column;
}

.request-details-pane {
  width: 70%;
  padding: 1rem;
  overflow-y: auto;
}

.context-menu {
  position: absolute;
  background-color: white;
  border: 1px solid #ccc;
  box-shadow: 2px 2px 5px rgba(0, 0, 0, 0.1);
  padding: 0;
  margin: 0;
  list-style: none;
}
.context-menu ul {
  list-style: none;
  padding: 0;
  margin: 0;
}
.context-menu li {
  padding: 0.5rem 1rem;
  cursor: pointer;
}
.context-menu li:hover {
  background-color: #f0f0f0;
}
.context-menu li.disabled {
  color: #ccc;
  cursor: not-allowed;
}
</style>