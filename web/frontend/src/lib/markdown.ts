// Shared markdown renderer for mail and review bodies. Combines marked
// with DOMPurify and forces every <a> to open in a new tab so users
// don't lose their inbox context when clicking through to issue or PR
// links embedded in agent messages.

import DOMPurify from 'dompurify';
import { marked } from 'marked';

// Module-level guard so we register the DOMPurify hook exactly once.
// Hooks accumulate across calls otherwise.
let hookRegistered = false;

// hasSetAttribute narrows an arbitrary node to one that exposes the
// setAttribute API. DOMPurify hooks see the underlying parsed DOM
// node, but the type signature uses the broad Node type and Element
// is not always statically inferred.
function hasSetAttribute(node: unknown): node is {
  tagName: string;
  setAttribute: (name: string, value: string) => void;
} {
  return (
    typeof node === 'object' &&
    node !== null &&
    'tagName' in node &&
    'setAttribute' in node &&
    typeof (node as { setAttribute: unknown }).setAttribute === 'function'
  );
}

// ensureExternalLinkHook registers an afterSanitizeAttributes hook
// that adds target="_blank" and rel="noopener noreferrer" to every
// anchor surviving sanitization. Idempotent.
function ensureExternalLinkHook(): void {
  if (hookRegistered) return;
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (hasSetAttribute(node) && node.tagName === 'A') {
      node.setAttribute('target', '_blank');
      node.setAttribute('rel', 'noopener noreferrer');
    }
  });
  hookRegistered = true;
}

// Tags allowed in rendered markdown. style is intentionally excluded
// to prevent CSS-based injection; the GFM column-alignment marker
// (:---:) is lost as a result, which is an accepted trade-off.
const ALLOWED_TAGS = [
  'p', 'br', 'strong', 'em', 'code', 'pre', 'ul', 'ol', 'li',
  'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'a', 'blockquote', 'hr',
  'table', 'thead', 'tbody', 'tr', 'th', 'td',
];

const ALLOWED_ATTR = ['href', 'target', 'rel'];

// renderMarkdownToHtml parses GFM markdown to HTML, sanitizes it, and
// rewrites anchors to open in a new tab.
export function renderMarkdownToHtml(text: string): string {
  ensureExternalLinkHook();
  const rawHtml = marked.parse(text, {
    async: false, gfm: true, breaks: true,
  }) as string;
  return DOMPurify.sanitize(rawHtml, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
  });
}
