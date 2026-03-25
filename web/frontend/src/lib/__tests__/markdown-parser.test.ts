import { describe, it, expect } from 'vitest';
import {
  parseMarkdownToBlocks,
  extractFrontmatter,
} from '../markdown-parser.js';

describe('extractFrontmatter', () => {
  it('should return null for content without frontmatter', () => {
    const result = extractFrontmatter('# Hello\nWorld');
    expect(result.frontmatter).toBeNull();
    expect(result.content).toBe('# Hello\nWorld');
  });

  it('should extract simple key-value frontmatter', () => {
    const md = '---\ntitle: My Plan\nauthor: Alice\n---\n# Hello';
    const result = extractFrontmatter(md);
    expect(result.frontmatter).toEqual({ title: 'My Plan', author: 'Alice' });
    expect(result.content).toBe('# Hello');
  });

  it('should extract array values in frontmatter', () => {
    const md = '---\ntags:\n- alpha\n- beta\n---\nContent';
    const result = extractFrontmatter(md);
    expect(result.frontmatter?.tags).toEqual(['alpha', 'beta']);
  });
});

describe('parseMarkdownToBlocks', () => {
  it('should parse headings at different levels', () => {
    const blocks = parseMarkdownToBlocks('# H1\n## H2\n### H3');
    expect(blocks).toHaveLength(3);
    expect(blocks[0]!.type).toBe('heading');
    expect(blocks[0]!.level).toBe(1);
    expect(blocks[0]!.content).toBe('H1');
    expect(blocks[1]!.type).toBe('heading');
    expect(blocks[1]!.level).toBe(2);
    expect(blocks[2]!.level).toBe(3);
  });

  it('should parse paragraphs', () => {
    const blocks = parseMarkdownToBlocks('Hello world.\n\nSecond paragraph.');
    expect(blocks).toHaveLength(2);
    expect(blocks[0]!.type).toBe('paragraph');
    expect(blocks[0]!.content).toBe('Hello world.');
    expect(blocks[1]!.type).toBe('paragraph');
    expect(blocks[1]!.content).toBe('Second paragraph.');
  });

  it('should parse list items with bullets', () => {
    const blocks = parseMarkdownToBlocks('- Item 1\n- Item 2\n- Item 3');
    expect(blocks).toHaveLength(3);
    blocks.forEach((b) => expect(b.type).toBe('list-item'));
    expect(blocks[0]!.content).toBe('Item 1');
  });

  it('should parse checkbox list items', () => {
    const blocks = parseMarkdownToBlocks('- [x] Done\n- [ ] Todo');
    expect(blocks).toHaveLength(2);
    expect(blocks[0]!.checked).toBe(true);
    expect(blocks[0]!.content).toBe('Done');
    expect(blocks[1]!.checked).toBe(false);
    expect(blocks[1]!.content).toBe('Todo');
  });

  it('should parse nested list items with indent levels', () => {
    const blocks = parseMarkdownToBlocks('- Top\n  - Nested\n    - Deep');
    expect(blocks).toHaveLength(3);
    expect(blocks[0]!.level).toBe(0);
    expect(blocks[1]!.level).toBe(1);
    expect(blocks[2]!.level).toBe(2);
  });

  it('should parse blockquotes', () => {
    const blocks = parseMarkdownToBlocks('> This is a quote');
    expect(blocks).toHaveLength(1);
    expect(blocks[0]!.type).toBe('blockquote');
    expect(blocks[0]!.content).toBe('This is a quote');
  });

  it('should parse code blocks with language', () => {
    const blocks = parseMarkdownToBlocks('```go\nfunc main() {}\n```');
    expect(blocks).toHaveLength(1);
    expect(blocks[0]!.type).toBe('code');
    expect(blocks[0]!.language).toBe('go');
    expect(blocks[0]!.content).toBe('func main() {}');
  });

  it('should parse code blocks without language', () => {
    const blocks = parseMarkdownToBlocks('```\nplain code\n```');
    expect(blocks).toHaveLength(1);
    expect(blocks[0]!.type).toBe('code');
    expect(blocks[0]!.language).toBeUndefined();
  });

  it('should handle nested code fences', () => {
    const md = '````\n```go\ninner\n```\n````';
    const blocks = parseMarkdownToBlocks(md);
    expect(blocks).toHaveLength(1);
    expect(blocks[0]!.type).toBe('code');
    expect(blocks[0]!.content).toContain('```go');
  });

  it('should parse horizontal rules', () => {
    const blocks = parseMarkdownToBlocks('Text\n\n---\n\nMore text');
    expect(blocks).toHaveLength(3);
    expect(blocks[1]!.type).toBe('hr');
  });

  it('should parse tables', () => {
    const md = '| A | B |\n| --- | --- |\n| 1 | 2 |';
    const blocks = parseMarkdownToBlocks(md);
    expect(blocks).toHaveLength(1);
    expect(blocks[0]!.type).toBe('table');
    expect(blocks[0]!.content).toContain('| A | B |');
  });

  it('should assign stable block IDs', () => {
    const blocks = parseMarkdownToBlocks('# A\n# B\n# C');
    expect(blocks[0]!.id).toBe('block-0');
    expect(blocks[1]!.id).toBe('block-1');
    expect(blocks[2]!.id).toBe('block-2');
  });

  it('should track start line numbers', () => {
    const blocks = parseMarkdownToBlocks('# H1\n\nParagraph\n\n- List');
    expect(blocks[0]!.startLine).toBe(1);
    expect(blocks[1]!.startLine).toBe(3);
    expect(blocks[2]!.startLine).toBe(5);
  });

  it('should handle empty input', () => {
    const blocks = parseMarkdownToBlocks('');
    expect(blocks).toHaveLength(0);
  });

  it('should strip frontmatter before parsing', () => {
    const md = '---\ntitle: Plan\n---\n# Heading';
    const blocks = parseMarkdownToBlocks(md);
    expect(blocks).toHaveLength(1);
    expect(blocks[0]!.type).toBe('heading');
    expect(blocks[0]!.content).toBe('Heading');
  });
});
