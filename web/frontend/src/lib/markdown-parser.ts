// Markdown-to-blocks parser for plan annotation.
// Ported from plannotator's parser with adaptations for substrate.
// Splits markdown into flat, predictable blocks with stable IDs for
// annotation anchoring.

import type { Block, BlockType } from '@/types/annotations.js';

// Frontmatter represents parsed YAML front matter as key-value pairs.
export interface Frontmatter {
  [key: string]: string | string[];
}

// extractFrontmatter extracts YAML front matter from markdown if present.
// Returns both the parsed front matter and the remaining content.
export function extractFrontmatter(
  markdown: string,
): { frontmatter: Frontmatter | null; content: string } {
  const trimmed = markdown.trimStart();
  if (!trimmed.startsWith('---')) {
    return { frontmatter: null, content: markdown };
  }

  const endIndex = trimmed.indexOf('\n---', 3);
  if (endIndex === -1) {
    return { frontmatter: null, content: markdown };
  }

  const frontmatterRaw = trimmed.slice(4, endIndex).trim();
  const afterFrontmatter = trimmed.slice(endIndex + 4).trimStart();

  const frontmatter: Frontmatter = {};
  let currentKey: string | null = null;
  let currentArray: string[] | null = null;

  for (const line of frontmatterRaw.split('\n')) {
    const trimmedLine = line.trim();

    // Array item (- value).
    if (trimmedLine.startsWith('- ') && currentKey) {
      const value = trimmedLine.slice(2).trim();
      if (!currentArray) {
        currentArray = [];
        frontmatter[currentKey] = currentArray;
      }
      currentArray.push(value);
      continue;
    }

    // Key: value pair.
    const colonIndex = trimmedLine.indexOf(':');
    if (colonIndex > 0) {
      currentKey = trimmedLine.slice(0, colonIndex).trim();
      const value = trimmedLine.slice(colonIndex + 1).trim();
      currentArray = null;

      if (value) {
        frontmatter[currentKey] = value;
      }
    }
  }

  return { frontmatter, content: afterFrontmatter };
}

// parseMarkdownToBlocks splits markdown content into a flat array of blocks.
// Each block has a stable ID, type, content, and source line number for
// reliable annotation anchoring.
export function parseMarkdownToBlocks(markdown: string): Block[] {
  const { content: cleanMarkdown } = extractFrontmatter(markdown);
  const lines = cleanMarkdown.split('\n');
  const blocks: Block[] = [];
  let currentId = 0;

  let buffer: string[] = [];
  let currentType: BlockType = 'paragraph';
  const currentLevel = 0;
  let bufferStartLine = 1;

  // flush pushes the accumulated buffer as a new block.
  const flush = () => {
    if (buffer.length > 0) {
      const content = buffer.join('\n');
      blocks.push({
        id: `block-${currentId++}`,
        type: currentType,
        content,
        level: currentLevel,
        order: currentId,
        startLine: bufferStartLine,
      });
      buffer = [];
    }
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]!;
    const trimmed = line.trim();
    const currentLineNum = i + 1;

    // Headings.
    if (trimmed.startsWith('#')) {
      flush();
      const level = trimmed.match(/^#+/)?.[0].length || 1;
      blocks.push({
        id: `block-${currentId++}`,
        type: 'heading',
        content: trimmed.replace(/^#+\s*/, ''),
        level,
        order: currentId,
        startLine: currentLineNum,
      });
      continue;
    }

    // Horizontal rules.
    if (trimmed === '---' || trimmed === '***') {
      flush();
      blocks.push({
        id: `block-${currentId++}`,
        type: 'hr',
        content: '',
        order: currentId,
        startLine: currentLineNum,
      });
      continue;
    }

    // List items — each item is a separate block for annotation targeting.
    if (trimmed.match(/^(\*|-|\d+\.)\s/)) {
      flush();
      const leadingWhitespace = line.match(/^(\s*)/)?.[1] || '';
      const spaceCount = leadingWhitespace.replace(/\t/g, '  ').length;
      const listLevel = Math.floor(spaceCount / 2);

      let content = trimmed.replace(/^(\*|-|\d+\.)\s/, '');

      // Check for checkbox syntax: [ ] or [x] or [X].
      const checkboxMatch = content.match(/^\[([ xX])\]\s*/);
      const isCheckbox = checkboxMatch !== null;
      const checked = isCheckbox
        ? checkboxMatch[1]!.toLowerCase() === 'x'
        : undefined;
      if (isCheckbox) {
        content = content.replace(/^\[([ xX])\]\s*/, '');
      }

      const block: Block = {
        id: `block-${currentId++}`,
        type: 'list-item',
        content,
        level: listLevel,
        order: currentId,
        startLine: currentLineNum,
      };
      if (checked !== undefined) {
        block.checked = checked;
      }
      blocks.push(block);
      continue;
    }

    // Blockquotes.
    if (trimmed.startsWith('>')) {
      flush();
      blocks.push({
        id: `block-${currentId++}`,
        type: 'blockquote',
        content: trimmed.replace(/^>\s*/, ''),
        order: currentId,
        startLine: currentLineNum,
      });
      continue;
    }

    // Code blocks with fence-length tracking for nested fences.
    if (trimmed.startsWith('```')) {
      flush();
      const codeStartLine = currentLineNum;
      const fenceLen = trimmed.match(/^`+/)?.[0].length ?? 3;
      const closingFence = new RegExp('^`{' + fenceLen + ',}\\s*$');
      const langStr = trimmed.slice(fenceLen).trim();

      const codeContent: string[] = [];
      i++;
      while (i < lines.length && !closingFence.test(lines[i]!)) {
        codeContent.push(lines[i]!);
        i++;
      }
      const codeBlock: Block = {
        id: `block-${currentId++}`,
        type: 'code',
        content: codeContent.join('\n'),
        order: currentId,
        startLine: codeStartLine,
      };
      if (langStr) {
        codeBlock.language = langStr;
      }
      blocks.push(codeBlock);
      continue;
    }

    // Tables — lines starting with or containing pipes.
    if (
      trimmed.startsWith('|') ||
      (trimmed.includes('|') &&
        trimmed.match(/^\|?.+\|.+\|?$/))
    ) {
      flush();
      const tableStartLine = currentLineNum;
      const tableLines: string[] = [line];

      while (i + 1 < lines.length) {
        const nextLine = lines[i + 1]!.trim();
        if (
          nextLine.startsWith('|') ||
          (nextLine.includes('|') &&
            nextLine.match(/^\|?.+\|.+\|?$/))
        ) {
          i++;
          tableLines.push(lines[i]!);
        } else {
          break;
        }
      }

      blocks.push({
        id: `block-${currentId++}`,
        type: 'table',
        content: tableLines.join('\n'),
        order: currentId,
        startLine: tableStartLine,
      });
      continue;
    }

    // Empty lines separate paragraphs.
    if (trimmed === '') {
      flush();
      currentType = 'paragraph';
      continue;
    }

    // Accumulate paragraph text.
    if (buffer.length === 0) {
      bufferStartLine = currentLineNum;
    }
    buffer.push(line);
  }

  flush();

  return blocks;
}
