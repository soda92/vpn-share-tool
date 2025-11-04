<template>
  <div class="diff-viewer">
    <table>
      <thead>
        <tr>
          <th>Field</th>
          <th>Request 1</th>
          <th>Request 2</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="field in diff" :key="field.key" :class="field.type">
          <td>{{ field.key }}</td>
          <td><pre>{{ field.value1 }}</pre></td>
          <td><pre>{{ field.value2 }}</pre></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  data1: Record<string, unknown>;
  data2: Record<string, unknown>;
}>();

const diff = computed(() => {
  const allKeys = new Set([...Object.keys(props.data1), ...Object.keys(props.data2)]);
  const result: { key: string; value1: unknown; value2: unknown; type: string }[] = [];

  for (const key of allKeys) {
    const value1 = props.data1[key];
    const value2 = props.data2[key];

    if (value1 === value2) {
      result.push({ key, value1, value2, type: 'unchanged' });
    } else if (key in props.data1 && !(key in props.data2)) {
      result.push({ key, value1, value2: undefined, type: 'removed' });
    } else if (!(key in props.data1) && key in props.data2) {
      result.push({ key, value1: undefined, value2, type: 'added' });
    } else {
      result.push({ key, value1, value2, type: 'modified' });
    }
  }
  return result;
});
</script>

<style scoped>
.diff-viewer table {
  width: 100%;
  border-collapse: collapse;
}
.diff-viewer th, .diff-viewer td {
  border: 1px solid #ccc;
  padding: 0.5rem;
  text-align: left;
}
.diff-viewer .added {
  background-color: #e6ffed;
}
.diff-viewer .removed {
  background-color: #ffeef0;
}
.diff-viewer .modified {
  background-color: #fff3cd;
}
pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
