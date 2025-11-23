<template>
  <div class="url-decoder">
    <input type="text" v-model="searchTerm" placeholder="Search by field name" />
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
import { ref, computed, defineProps } from 'vue';

const props = defineProps<{
  encodedData: string;
}>();

const searchTerm = ref('');

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
</script>

<style scoped>
.url-decoder {
  margin-top: 0.5rem;
  background-color: #fff;
  border: 1px solid #ddd;
  border-radius: 6px;
  padding: 1rem;
}

input {
  width: 100%;
  padding: 0.5rem;
  font-size: 0.9rem;
  margin-bottom: 0.5rem;
  border: 1px solid #ccc;
  border-radius: 4px;
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
