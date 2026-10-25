import { describe, expect, it } from 'vitest'
import { buildTree, flattenTree } from './tree'

describe('tree utilities', () => {
  const items = [
    { id: 3, title: '根 B', parentID: null, sort: 2 },
    { id: 2, title: '子 B', parentID: 1, sort: 2 },
    { id: 1, title: '根 A', parentID: null, sort: 1 },
    { id: 4, title: '子 A', parentID: 1, sort: 1 },
    { id: 5, title: '孤儿', parentID: 99, sort: 0 }
  ]

  it('builds sorted roots and children', () => {
    const roots = buildTree(items)
    expect(roots.map((node) => node.id)).toEqual([5, 1, 3])
    expect(roots[1].children.map((node) => node.id)).toEqual([4, 2])
  })

  it('flattens with depth and handles empty input', () => {
    const flat = flattenTree(buildTree(items))
    expect(flat.map((node) => [node.id, node.depth])).toEqual([
      [5, 0],
      [1, 0],
      [4, 1],
      [2, 1],
      [3, 0]
    ])
    expect(buildTree([])).toEqual([])
    expect(flattenTree()).toEqual([])
  })
})
