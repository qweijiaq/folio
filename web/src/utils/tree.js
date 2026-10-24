export function buildTree(items = []) {
  const map = new Map()
  items.forEach((item) => map.set(item.id, { ...item, children: [] }))
  const roots = []
  map.forEach((node) => {
    if (node.parentID && map.has(node.parentID)) {
      map.get(node.parentID).children.push(node)
    } else {
      roots.push(node)
    }
  })
  const sortNodes = (nodes) => {
    nodes.sort((a, b) => (a.sort || 0) - (b.sort || 0) || a.id - b.id)
    nodes.forEach((node) => sortNodes(node.children))
  }
  sortNodes(roots)
  return roots
}

export function flattenTree(nodes = [], depth = 0, out = []) {
  nodes.forEach((node) => {
    out.push({ ...node, depth })
    flattenTree(node.children || [], depth + 1, out)
  })
  return out
}
