<template>
  <div class="url-decoder">
    <div class="decoder-controls">
      <input type="text" v-model="searchTerm" placeholder="Search field..." class="search-input" />
      <div class="action-buttons">
        <button @click="showJson = !showJson" class="secondary-btn">{{ showJson ? 'Hide JSON' : 'View JSON' }}</button>
        <button v-if="showJson" @click="copyJson" class="secondary-btn">Copy</button>
      </div>
    </div>
    
    <pre v-if="showJson" class="json-output">{{ jsonOutput }}</pre>
    
    <table>
      <thead>
        <tr>
          <th>Field Name</th>
          <th>Value</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in filteredDecodedData" :key="item.id">
          <td>{{ item.key }}</td>
          <td>{{ item.value }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { useToast } from 'vue-toastification';

const props = defineProps<{
  encodedData: string;
}>();

const searchTerm = ref('');
const showJson = ref(false);
const toast = useToast();

const decodedData = computed(() => {
  if (!props.encodedData) {
    return [];
  }
  try {
    const searchParams = new URLSearchParams(props.encodedData);
    const data = [];
    let index = 0;
    for (const [key, value] of searchParams.entries()) {
      data.push({ id: `${key}-${index}`, key, value });
      index++;
    }
    return data;
  } catch (error) {
    console.error('Error decoding data:', error);
    return [];
  }
});

const filteredDecodedData = computed(() => {
  if (!searchTerm.value) {
    return decodedData.value;
  }
  return decodedData.value.filter(item =>
    item.key.toLowerCase().includes(searchTerm.value.toLowerCase())
  );
});

const parseValue = (val: string): any => {
  if (val === 'true') return true;
  if (val === 'false') return false;
  if (val === 'null') return null;
  if (val === 'undefined') return undefined;
  // Check for number (and not just empty string or whitespace)
  if (!isNaN(Number(val)) && val.trim() !== '') return Number(val);
  try {
    const json = JSON.parse(val);
    if (typeof json === 'object' && json !== null) return json;
  } catch (e) {
    // Ignore JSON parse error, keep as string
  }
  return val;
};

const jsonOutput = computed(() => {
  const obj: { [key: string]: any } = {};
  for (const { key, value } of decodedData.value) {
    const parsedValue = parseValue(value);
    if (obj.hasOwnProperty(key)) {
      if (Array.isArray(obj[key])) {
        (obj[key] as any[]).push(parsedValue);
      } else {
        obj[key] = [obj[key], parsedValue];
      }
    } else {
      obj[key] = parsedValue;
    }
  }
  return JSON.stringify(obj, null, 2);
});

const copyJson = () => {
  navigator.clipboard.writeText(jsonOutput.value)
    .then(() => {
      toast.success('JSON copied to clipboard!');
    })
    .catch(err => {
      console.error('Failed to copy JSON: ', err);
      toast.error('Failed to copy JSON.');
    });
};
</script>

<style scoped>
.url-decoder {
  margin-top: 0.5rem;
  background-color: #fff;
  border: 1px solid #ddd;
  border-radius: 6px;
  padding: 1rem;
}

.decoder-controls {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
  align-items: center;
  flex-wrap: wrap;
}

.search-input {
  flex-grow: 1;
  padding: 0.4rem;
  font-size: 0.9rem;
  border: 1px solid #ccc;
  border-radius: 4px;
  min-width: 150px;
}

.action-buttons {
  display: flex;
  gap: 0.5rem;
}

.secondary-btn {
  background-color: #6c757d;
  color: white;
  border: none;
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.85rem;
  white-space: nowrap;
}

.secondary-btn:hover {
  background-color: #5a6268;
}

.json-output {
  background-color: #f8f9fa;
  padding: 0.8rem;
  border: 1px solid #eee;
  border-radius: 4px;
  overflow-x: auto;
  font-size: 0.85rem;
  margin-bottom: 1rem;
}

table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed; /* Important for overflow control */
}

th, td {
  border: 1px solid #eee;
  padding: 0.5rem;
  text-align: left;
  font-size: 0.9rem;
  overflow: hidden;
  text-overflow: ellipsis;
}

td {
  white-space: pre-wrap; /* Allow wrapping for long values */
  word-break: break-all;
  font-family: 'Fira Code', 'Courier New', monospace;
}

th {
  background-color: #f8f9fa;
  width: 30%; /* Give keys less space than values */
  color: #555;
}
</style>
