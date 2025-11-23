<template>
  <div>
    <h1>Main Page</h1>
    <p>Welcome!</p>
    <button @click="logout">Logout</button>

    <hr>
    <h2>Test Form Submission</h2>
    <form @submit.prevent="submitForm">
      <input type="text" name="field1" v-model="formData.field1" placeholder="Field 1">
      <input type="text" name="field2" v-model="formData.field2" placeholder="Field 2">
      <button type="submit">Submit Form (x-www-form-urlencoded)</button>
    </form>
  </div>
</template>

<script>
export default {
  data() {
    return {
      formData: {
        field1: '',
        field2: ''
      }
    };
  },
  methods: {
    async logout() {
      try {
        const response = await fetch('/api/logout', { method: 'POST' });
        if (response.ok) {
          this.$router.push('/login');
        } else {
          console.error('Logout failed');
        }
      } catch (error) {
        console.error('Error during logout:', error);
      }
    },
    async submitForm() {
      const params = new URLSearchParams();
      params.append('field1', this.formData.field1);
      params.append('field2', this.formData.field2);
      params.append('extra', 'hidden value');

      try {
        await fetch('/api/submit', {
          method: 'POST',
          body: params,
          // Content-Type header is automatically set by fetch when body is URLSearchParams
        });
        alert('Form submitted!');
      } catch (e) {
        console.error('Form submission failed', e);
      }
    }
  },
  async created() {
    try {
      const response = await fetch('/api/check-auth');
      if (!response.ok) {
        this.$router.push('/login');
      }
    } catch (error) {
      console.error('Error checking auth:', error);
      this.$router.push('/login');
    }
  },
};
</script>
