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

      <div v-if="queryString">
        <h3>Query Parameters</h3>
        <UrlDecoder :encodedData="queryString" />
      </div>

      <h3>Notes</h3>
      <textarea :value="note" @input="$emit('update:note', ($event.target as HTMLTextAreaElement).value)" placeholder="Add notes here..."></textarea>

      <h3>Request Headers</h3>
      <pre>{{ request.request_headers }}</pre>

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

      <h3>Response Headers</h3>
      <pre>{{ request.response_headers }}</pre>

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
</style>
