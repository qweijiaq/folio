<template>
  <div class="article-skeleton container" aria-hidden="true">
    <div class="article-skeleton-head">
      <span class="skeleton-line skeleton-line--kicker"></span>
      <span class="skeleton-line skeleton-line--headline"></span>
      <span class="skeleton-line skeleton-line--headline skeleton-line--headline-short"></span>
      <span class="skeleton-line skeleton-line--abstract"></span>
      <div class="article-skeleton-byline">
        <span class="skeleton-avatar"></span>
        <span class="skeleton-line skeleton-line--meta"></span>
      </div>
    </div>
    <div class="skeleton-cover article-skeleton-cover"></div>
    <div class="article-skeleton-body">
      <span v-for="i in 7" :key="i" class="skeleton-line" :class="{ 'skeleton-line--short': i === 3 || i === 6 }"></span>
    </div>
  </div>
</template>
