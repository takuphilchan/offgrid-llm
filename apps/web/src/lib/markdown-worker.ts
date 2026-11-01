import { unified } from 'unified';
import remarkParse from 'remark-parse';
import remarkGfm from 'remark-gfm';
import remarkRehype from 'remark-rehype';
import type { Root, RootContent, Element } from 'hast';

// Match the renderer's Markdown dialect. Raw HTML is retained as text, never
// interpreted; no plugin downloads, remote images or arbitrary HTML execution.
const processor = unified().use(remarkParse).use(remarkGfm).use(remarkRehype, { allowDangerousHtml: true });
function inert(node: Root | RootContent): Root | RootContent {
  delete node.position;
  if ((node as { type: string }).type === 'raw') return { type: 'text', value: (node as unknown as { value: string }).value };
  if (node.type === 'element') {
    for (const name of ['href', 'src']) if (Object.hasOwn(node.properties, name)) {
      const value = String(node.properties[name] ?? '');
      node.properties[name] = /^(https?:|mailto:)/i.test(value.trim()) ? value : '';
    }
  }
  if ('children' in node) node.children = node.children.map(child => inert(child)) as Element['children'];
  return node;
}
self.onmessage = (event: MessageEvent<{ id: number; content: string }>) => {
  const { id, content } = event.data;
  try {
    const tree = processor.runSync(processor.parse(content)) as Root;
    self.postMessage({ id, tree: inert(tree) });
  } catch {
    // The UI keeps the exact source visible if parsing cannot complete.
    self.postMessage({ id, failed: true });
  }
};
