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
      <UrlDecoder
        v-if="isWwwFormUrlEncoded"
        :encodedData="request.request_body"
      />
      <pre v-else>{{ request.request_body }}</pre>

      <h3>Response Headers</h3>
      <pre>{{ request.response_headers }}</pre>

      <h3>Response Body</h3>
      <div v-if="isImage && request.is_base64" class="image-preview">
        <img :src="imageSrc" alt="Response Image" style="max-width: 100%; border: 1px solid #ddd; border-radius: 4px;">
      </div>
      <pre v-else-if="isJsonResponse">{{ formattedResponseBody }}</pre>
      <pre v-else>{{ request.response_body }}</pre>
    </div>
    <div v-else class="no-selection">
      Select a request to see details.
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineProps, defineEmits } from 'vue';
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
  if (!props.request || !isImage.value) return '';
  const contentType = props.request.response_headers['Content-Type']?.[0] || 'image/png';
  return `data:${contentType};base64,${props.request.response_body}`;
});

const formattedResponseBody = computed(() => {
  if (props.request && isJsonResponse.value) {
    try {
      const jsonObj = JSON.parse(props.request.response_body);
      return JSON.stringify(jsonObj, null, 2);
    } catch {
      return props.request.response_body;
    }
  }
  return props.request?.response_body;
});

const queryString = computed(() => {
  if (!props.request?.url) return '';
  const qIndex = props.request.url.indexOf('?');
  if (qIndex === -1) return '';
  return props.request.url.substring(qIndex + 1);
});
</script>

<style scoped>
.request-details-pane {
  width: 100%; /* Fill parent width */
  height: 100%; /* Fill parent height (desktop) */
  padding: 1.5rem;
  overflow-y: auto;
  background-color: #f5f5f5;
  display: block; /* Use block layout for simple scrolling */
}

.mobile-header {
  display: none;
  padding-bottom: 1rem;
  margin-bottom: 1rem;
  border-bottom: 1px solid #ddd;
}

.back-btn {
  background: none;
  border: none;
  color: #007bff;
  font-size: 1rem;
  cursor: pointer;
  padding: 0;
  font-weight: bold;
}

@media (max-width: 768px) {
  .request-details-pane {
    width: 100%;
    padding: 0.5rem;
    overflow: visible; /* Let window scroll */
    height: auto;
  }
  
  .mobile-header {
    display: block;
    margin-bottom: 0.5rem;
    padding-bottom: 0.5rem;
  }
  
  .details-grid {
    grid-template-columns: 80px 1fr; /* Even smaller label col */
    gap: 0.5rem;
    padding: 0.5rem;
  }
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

textarea {
  width: 100%;
  min-height: 100px;
  padding: 0.5rem;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-family: inherit;
}
</style>
