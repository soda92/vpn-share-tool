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
        <tr v-for="(item, index) in filteredDecodedData" :key="index">
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
    for (const [key, value] of searchParams.entries()) {
      data.push({ key, value });
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
  margin-top: 1rem;
}

input {
  width: 100%;
  padding: 0.5rem;
  font-size: 1rem;
  margin-bottom: 1rem;
}

table {
  width: 100%;
  border-collapse: collapse;
}

th, td {
  border: 1px solid #ddd;
  padding: 0.5rem;
  text-align: left;
}

th {
  background-color: #f2f2f2;
}
</style>
