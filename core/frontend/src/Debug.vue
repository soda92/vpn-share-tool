<template>
  <div id="app">
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
            <button @click="compare" :disabled="selectedForCompare.length !== 2">Compare</button>
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
            :class="{ selected: selectedRequest && selectedRequest.id === request.id }"
          >
            <input
              v-if="request.method === 'POST'"
              type="checkbox"
              :value="request.id"
              v-model="selectedForCompare"
              @click.stop
            />
            <span class="method">{{ request.method }}</span>
            <span class="url">{{ request.url }}</span>
          </li>
        </ul>
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
const selectedForCompare = ref<number[]>([]);

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
      selectedForCompare.value = [];
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

const compare = () => {
  if (selectedForCompare.value.length === 2) {
    const req1 = requests.value.find(r => r.id === selectedForCompare.value[0]);
    const req2 = requests.value.find(r => r.id === selectedForCompare.value[1]);

    if (req1 && req2) {
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
  }
};

onMounted(() => {
  fetchRequests();
  setInterval(fetchRequests, 2000); // Poll for new requests every 2 seconds
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

.request-list-header {
  display: flex;
  flex-direction: column;
  padding: 1rem;
  border-bottom: 1px solid #ccc;
}

.filter-controls {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.request-list {
  list-style: none;
  padding: 0;
  margin: 0;
  overflow-y: auto;
  flex-grow: 1;
}

.request-list li {
  padding: 0.5rem 1rem;
  cursor: pointer;
  border-bottom: 1px solid #eee;
  display: flex;
  gap: 1rem;
  align-items: center;
}

.request-list li:hover {
  background-color: #f0f0f0;
}

.request-list li.selected {
  background-color: #e0e0ff;
}

.method {
  font-weight: bold;
  width: 50px;
}

.url {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.no-requests,
.no-selection {
  padding: 1rem;
  text-align: center;
  color: #888;
}

.request-details-pane {
  width: 70%;
  padding: 1rem;
  overflow-y: auto;
}

.details-grid {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.5rem 1rem;
  margin-bottom: 1rem;
}

h3 {
  margin-top: 2rem;
  border-bottom: 1px solid #ccc;
  padding-bottom: 0.5rem;
}

pre {
  background-color: #f5f5f5;
  padding: 1rem;
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
