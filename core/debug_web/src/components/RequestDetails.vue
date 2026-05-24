<template>
  <div class="request-details-pane">
    <div class="mobile-header">
      <button class="back-btn" @click="$emit('close')">← Back</button>
    </div>
    <div v-if="request">
      <h2>Request Details</h2>
      <div class="details-grid">
        <div><strong>URL:</strong></div>
        <div>{{ request.url }}</div>
        <div><strong>Method:</strong></div>
        <div>{{ request.method }}</div>
        <div><strong>Status:</strong></div>
        <div>{{ request.response_status }}</div>
        <div><strong>Timestamp:</strong></div>
        <div>{{ new Date(request.timestamp).toLocaleString() }}</div>
      </div>

      <!-- Request Body -->
      <h3>Request Body</h3>
      <div v-if="request.request_body === undefined" class="loading-body">
        Loading body...
      </div>
      <template v-else>
        <UrlDecoder
          v-if="isWwwFormUrlEncoded"
          :encodedData="request.request_body"
        />
        <pre v-else>{{ request.request_body }}</pre>
      </template>

      <!-- Response Body -->
      <h3>Response Body</h3>
      <div v-if="request.response_body === undefined" class="loading-body">
        Loading body...
      </div>
      <template v-else>
        <div v-if="isImage && request.is_base64" class="image-preview">
          <img :src="imageSrc" alt="Response Image" style="max-width: 100%; border: 1px solid #ddd; border-radius: 4px;">
        </div>
        <pre v-else-if="isJsonResponse">{{ formattedResponseBody }}</pre>
        <pre v-else>{{ request.response_body }}</pre>
      </template>

      <!-- Collapsible details sections at the bottom -->
      <div class="collapsible-sections">
        <details v-if="queryString">
          <summary>Query Parameters</summary>
          <div class="details-content">
            <UrlDecoder :encodedData="queryString" />
          </div>
        </details>

        <details>
          <summary>Notes</summary>
          <div class="details-content">
            <textarea :value="note" @input="$emit('update:note', ($event.target as HTMLTextAreaElement).value)" placeholder="Add notes here..."></textarea>
          </div>
        </details>

        <details>
          <summary>Request Headers</summary>
          <div class="details-content">
            <pre class="headers-pre">{{ request.request_headers }}</pre>
          </div>
        </details>

        <details>
          <summary>Response Headers</summary>
          <div class="details-content">
            <pre class="headers-pre">{{ request.response_headers }}</pre>
          </div>
        </details>
      </div>
    </div>
    <div v-else class="no-selection">
      Select a request to see details.
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed} from 'vue';
import type { CapturedRequest } from '../types';
import UrlDecoder from './UrlDecoder.vue';

const props = defineProps<{
  request: CapturedRequest | null;
  note: string;
}>();

defineEmits<{
  (e: 'update:note', value: string): void;
  (e: 'close'): void;
}>();

const isWwwFormUrlEncoded = computed(() => {
  if (!props.request) return false;
  const contentType = props.request.request_headers['Content-Type']?.[0] || '';
  return contentType.includes('application/x-www-form-urlencoded');
});

const isJsonResponse = computed(() => {
  if (!props.request) return false;
  const contentType = props.request.response_headers['Content-Type']?.[0] || '';
  return contentType.includes('application/json');
});

const isImage = computed(() => {
  if (!props.request) return false;
  const contentType = props.request.response_headers['Content-Type']?.[0] || '';
  return contentType.startsWith('image/');
});

const imageSrc = computed(() => {
  const body = props.request?.response_body;
  if (!props.request || !isImage.value || !body) return '';
  const contentType = props.request.response_headers['Content-Type']?.[0] || 'image/png';
  return `data:${contentType};base64,${body}`;
});

const formattedResponseBody = computed(() => {
  const body = props.request?.response_body;
  if (body && isJsonResponse.value) {
    try {
      const jsonObj = JSON.parse(body);
      return JSON.stringify(jsonObj, null, 2);
    } catch {
      return body;
    }
  }
  return body;
});

const queryString = computed(() => {
  if (!props.request?.url) return '';
  try {
    const url = new URL(props.request.url);
    return url.searchParams.toString();
  } catch (e) {
    // Fallback for invalid URLs.
    const qIndex = props.request.url.indexOf('?');
    if (qIndex === -1) return '';
    return props.request.url.substring(qIndex + 1);
  }
});
</script>

<style scoped>
.request-details-pane {
  width: 100%;
  height: 100%;
  padding: 1.5rem;
  overflow-y: auto;
  background-color: #fcfcfc;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.request-details-pane > div:not(.mobile-header) {
  width: 100%;
  max-width: 1200px;
}

.mobile-header {
  display: none;
  padding-bottom: 1rem;
  margin-bottom: 1rem;
  border-bottom: 1px solid #f0f0f0;
}

.back-btn {
  background: none;
  border: none;
  color: #673ab7;
  font-size: 1rem;
  cursor: pointer;
  padding: 0;
  font-weight: 700;
}

@media (max-width: 768px) {
  .request-details-pane {
    width: 100%;
    padding: 0.75rem;
    overflow: visible;
    height: auto;
  }
  
  .mobile-header {
    display: block;
    margin-bottom: 0.5rem;
    padding-bottom: 0.5rem;
  }
  
  .details-grid {
    grid-template-columns: 80px 1fr;
    gap: 0.5rem;
    padding: 0.75rem;
  }
}

.details-grid {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 0.8rem;
  margin-bottom: 1.5rem;
  background-color: #fff;
  padding: 1rem 1.2rem;
  border-radius: 8px;
  border: 1px solid #eee;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.details-grid div {
  word-break: break-all;
  font-size: 0.9rem;
}

.details-grid div strong {
  color: #555;
  font-weight: 600;
}

h2 {
  font-size: 1.25rem;
  font-weight: 700;
  color: #673ab7;
  margin-bottom: 1rem;
}

h3 {
  font-size: 1rem;
  font-weight: 700;
  margin-top: 1.5rem;
  margin-bottom: 0.75rem;
  color: #455a64;
  border-bottom: 1.5px solid #673ab7;
  padding-bottom: 0.3rem;
  display: inline-block;
}

pre {
  background-color: #fafafa;
  padding: 1rem;
  border: 1.5px solid #e0e0e0;
  border-radius: 8px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'Fira Code', 'Courier New', Courier, monospace;
  font-size: 0.85rem;
  color: #37474f;
  margin-bottom: 1rem;
}

.no-selection {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  font-size: 1.1rem;
  color: #78909c;
  font-style: italic;
}

textarea {
  width: 100%;
  min-height: 100px;
  padding: 0.6rem 0.8rem;
  border: 1.5px solid #e0e0e0;
  border-radius: 8px;
  font-family: inherit;
  font-size: 0.9rem;
  outline: none;
  background-color: #fff;
  transition: border-color 0.2s, box-shadow 0.2s;
  resize: vertical;
}

textarea:focus {
  border-color: #673ab7;
  box-shadow: 0 0 0 3px rgba(103, 58, 183, 0.15);
}

.loading-body {
  padding: 1rem;
  background-color: #fafafa;
  border: 1.5px solid #e0e0e0;
  border-radius: 8px;
  color: #757575;
  font-style: italic;
  width: 100%;
  font-size: 0.85rem;
  text-align: center;
}

.collapsible-sections {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-top: 2rem;
  width: 100%;
}

details {
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  background-color: #fafafa;
  overflow: hidden;
  transition: all 0.2s ease;
  width: 100%;
}

details[open] {
  background-color: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  border-color: #673ab7;
}

summary {
  padding: 0.6rem 1rem;
  cursor: pointer;
  font-weight: 700;
  font-size: 0.9rem;
  color: #455a64;
  outline: none;
  user-select: none;
  transition: background-color 0.2s;
  border-bottom: 1px solid transparent;
}

details[open] summary {
  border-bottom-color: #f0f0f0;
  background-color: #f3e5f5;
  color: #673ab7;
}

summary:hover {
  background-color: #f5f5f5;
}

.details-content {
  padding: 1rem;
  background-color: #fff;
}

.headers-pre {
  margin: 0;
  border: none;
  background-color: #fff;
  padding: 0;
}
</style>
