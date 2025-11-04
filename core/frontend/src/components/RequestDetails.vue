<template>
  <div class="request-details-pane">
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

      <h3>Notes</h3>
      <textarea :value="note" @input="$emit('update:note', $event.target.value)" placeholder="Add notes here..."></textarea>

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
      <pre v-if="isJsonResponse">{{ formattedResponseBody }}</pre>
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
</script>
