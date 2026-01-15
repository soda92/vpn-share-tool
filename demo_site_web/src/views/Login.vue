<template>
  <div>
    <h1>Login Page</h1>
    <form @submit.prevent="login">
      <input type="text" v-model="username" placeholder="Username" />
      <input type="password" v-model="password" placeholder="Password" />
      <button type="submit">Login</button>
    </form>
    <dt id="dd65-ddheader" onclick="Ehr.phisIndex()">健康档案</dt>

    <div style="margin-top: 20px; border-top: 1px solid #ccc; padding-top: 10px;">
      <h3>Debug Links</h3>
      <ul>
        <li><a href="/?simple=value&num=123&bool=true">Simple Params</a></li>
        <li><a href="/?data=%7B%22id%22%3A1%2C%22name%22%3A%22test%22%7D&list=1&list=2">JSON & Array Params</a></li>
      </ul>
    </div>
  </div>
</template>

<script>
export default {
  data() {
    return {
      username: '',
      password: '',
    };
  },
  methods: {
    async login() {
      try {
        const response = await fetch('/api/login', { method: 'POST' });
        if (response.ok) {
          this.$router.push('/');
        } else {
          console.error('Login failed');
        }
      } catch (error) {
        console.error('Error during login:', error);
      }
    },
  },
};
</script>