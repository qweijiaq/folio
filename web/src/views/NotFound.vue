<script setup>
import EmptyState from '../components/EmptyState.vue'
</script>

<template>
  <div class="page notfound-page container">
    <EmptyState icon="info" title="这一页不存在" description="链接可能已经失效，或者内容被移到了别处。">
      <div class="hero-actions">
        <RouterLink to="/" class="btn btn-primary btn-sm">回到首页</RouterLink>
        <RouterLink to="/articles" class="btn btn-ghost btn-sm">浏览文章</RouterLink>
      </div>
    </EmptyState>
  </div>
</template>
