<!--
- Copyright (c) 2024-present, pull-vert-contributors.
-
- This source code is licensed under the MIT license found in the
- LICENSE file in the root directory of this source tree.
-->

<template>
  <div id="app">
    <div class="main-layout">
      <div class="request-list-pane">
        <div class="request-list-header">
          <h2>Requests</h2>
          <button @click="clearHistory">Clear</button>
        </div>
        <div v-if="requests.length === 0" class="no-requests">
          No requests captured yet.
        </div>
        <ul v-else class="request-list">
          <li
            v-for="request in requests"
            :key="request.id"
            @click="selectRequest(request)"
            :class="{ selected: selectedRequest && selectedRequest.id === request.id }"
          >
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
          <pre>{{ selectedRequest.request_body }}</pre>

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
import { ref, onMounted } from 'vue';

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
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  border-bottom: 1px solid #ccc;
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
