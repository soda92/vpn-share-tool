<template>
  <div>
    <h1>Login Page</h1>
    <form @submit.prevent="login">
      <input type="text" v-model="username" placeholder="Username" />
      <input type="password" v-model="password" placeholder="Password" />
      <button type="submit">Login</button>
    </form>
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