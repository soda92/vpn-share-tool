<template>
  <div>
    <h1>Main Page</h1>
    <p>Welcome!</p>
    <button @click="logout">Logout</button>
  </div>
</template>

<script>
export default {
  methods: {
    async logout() {
      try {
        const response = await fetch('/logout', { method: 'POST' });
        if (response.ok) {
          this.$router.push('/login');
        } else {
          console.error('Logout failed');
        }
      } catch (error) {
        console.error('Error during logout:', error);
      }
    },
  },
  async created() {
    try {
      const response = await fetch('/check-auth');
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
