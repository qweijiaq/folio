import { describe, expect, it, vi } from 'vitest'
import { formatBytes, formatDate, formatNumber, readingTime, roleLabel, shortDate, timeAgo } from './format'

describe('format utilities', () => {
  it('formats dates and invalid values', () => {
    expect(formatDate('2024-03-05T12:00:00')).toBe('2024 年 3 月 5 日')
    expect(shortDate('2024-03-05T12:00:00')).toBe('2024.03.05')
    expect(formatDate('not-a-date')).toBe('')
    expect(shortDate('')).toBe('')
  })

  it('describes relative time', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2024-03-05T12:00:00Z'))
    expect(timeAgo('2024-03-05T11:59:30Z')).toBe('刚刚')
    expect(timeAgo('2024-03-05T11:55:00Z')).toBe('5 分钟前')
    expect(timeAgo('2024-03-05T10:00:00Z')).toBe('2 小时前')
    expect(timeAgo('2024-03-02T12:00:00Z')).toBe('3 天前')
    expect(timeAgo('2023-12-01T12:00:00Z')).toBe('2023.12.01')
    vi.useRealTimers()
  })

  it('shortens numbers and bytes', () => {
    expect(formatNumber(999)).toBe('999')
    expect(formatNumber(1200)).toBe('1.2k')
    expect(formatNumber(10000)).toBe('1w')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1024)).toBe('1.0 KB')
    expect(formatBytes(1536)).toBe('1.5 KB')
    expect(formatBytes(10 * 1024)).toBe('10 KB')
    expect(formatBytes(1024 * 1024)).toBe('1.0 MB')
  })

  it('estimates reading time and labels roles', () => {
    expect(readingTime('')).toBe(1)
    expect(readingTime('字'.repeat(800))).toBe(2)
    expect(roleLabel('super_admin')).toBe('超级管理员')
    expect(roleLabel('admin')).toBe('管理员')
    expect(roleLabel('user')).toBe('读者')
  })
})
