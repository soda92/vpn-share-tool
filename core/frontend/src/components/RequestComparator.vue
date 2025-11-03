<template>
  <div class="comparator-layout">
    <div class="request-pane">
      <h2>Request 1</h2>
      <pre>{{ req1 }}</pre>
    </div>
    <div class="request-pane">
      <h2>Request 2</h2>
      <pre>{{ req2 }}</pre>
    </div>
    <div class="diff-pane">
      <h2>Diff</h2>
      <DiffViewer v-if="diffData" :data1="diffData.data1" :data2="diffData.data2" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useRoute } from 'vue-router';
import DiffViewer from './DiffViewer.vue';

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

const route = useRoute();
const req1 = ref<CapturedRequest | null>(null);
const req2 = ref<CapturedRequest | null>(null);

const fetchRequest = async (id: number): Promise<CapturedRequest | null> => {
  try {
    const response = await fetch(`/debug/requests`);
    if (response.ok) {
      const requests = await response.json();
      return requests.find((r: CapturedRequest) => r.id === id) || null;
    } else {
      console.error(`Failed to fetch request ${id}`);
      return null;
    }
  } catch (error) {
    console.error(`Error fetching request ${id}:`, error);
    return null;
  }
};

const diffData = computed(() => {
  if (!req1.value || !req2.value) {
    return null;
  }

  const req1Body = req1.value.request_body;
  const req2Body = req2.value.request_body;

  const req1ContentType = req1.value.request_headers['Content-Type']?.[0] || '';
  const isJson = req1ContentType.includes('application/json');
  const isForm = req1ContentType.includes('application/x-www-form-urlencoded');

  let data1: Record<string, any> = {};
  let data2: Record<string, any> = {};

  if (isJson) {
    data1 = JSON.parse(req1Body);
    data2 = JSON.parse(req2Body);
  } else if (isForm) {
    data1 = Object.fromEntries(new URLSearchParams(req1Body));
    data2 = Object.fromEntries(new URLSearchParams(req2Body));
  }

  return { data1, data2 };
});

onMounted(async () => {
  const req1Id = Number(route.query.req1);
  const req2Id = Number(route.query.req2);

  if (req1Id) {
    req1.value = await fetchRequest(req1Id);
  }
  if (req2Id) {
    req2.value = await fetchRequest(req2Id);
  }
});
</script>

<style scoped>
.comparator-layout {
  display: flex;
  height: 100vh;
}
.request-pane, .diff-pane {
  width: 33.3%;
  padding: 1rem;
  overflow-y: auto;
  border-right: 1px solid #ccc;
}
.diff-pane {
  border-right: none;
}
pre {
  background-color: #f5f5f5;
  padding: 1rem;
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>