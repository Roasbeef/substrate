// Feedback export utilities for converting annotations into structured
// markdown feedback sent back to agents. Ported from plannotator's
// parser.ts (exportAnnotations) and exportFeedback.ts.

import type {
  Block,
  PlanAnnotation,
  DiffAnnotation,
} from '@/types/annotations.js';

// =============================================================================
// Plan Annotation Export
// =============================================================================

// exportPlanAnnotations formats plan annotations into structured markdown
// suitable for sending back to an agent as feedback. Annotations are sorted
// by block order and then by offset within each block.
export function exportPlanAnnotations(
  blocks: Block[],
  annotations: PlanAnnotation[],
): string {
  if (annotations.length === 0) {
    return 'No changes detected.';
  }

  const sortedAnns = [...annotations].sort((a, b) => {
    const blockA = blocks.findIndex((blk) => blk.id === a.blockId);
    const blockB = blocks.findIndex((blk) => blk.id === b.blockId);
    if (blockA !== blockB) return blockA - blockB;
    return a.startOffset - b.startOffset;
  });

  let output = `# Plan Feedback\n\n`;
  output += `I've reviewed this plan and have ${annotations.length} piece${
    annotations.length > 1 ? 's' : ''
  } of feedback:\n\n`;

  sortedAnns.forEach((ann, index) => {
    output += `## ${index + 1}. `;

    if (ann.diffContext) {
      output += `[In diff content] `;
    }

    switch (ann.type) {
      case 'DELETION':
        output += `Remove this\n`;
        output += `\`\`\`\n${ann.originalText}\n\`\`\`\n`;
        output += `> I don't want this in the plan.\n`;
        break;

      case 'INSERTION':
        output += `Add this\n`;
        output += `\`\`\`\n${ann.text}\n\`\`\`\n`;
        break;

      case 'REPLACEMENT':
        output += `Change this\n`;
        output += `**From:**\n\`\`\`\n${ann.originalText}\n\`\`\`\n`;
        output += `**To:**\n\`\`\`\n${ann.text}\n\`\`\`\n`;
        break;

      case 'COMMENT':
        output += `Feedback on: "${ann.originalText}"\n`;
        output += `> ${ann.text}\n`;
        break;

      case 'GLOBAL_COMMENT':
        output += `General feedback about the plan\n`;
        output += `> ${ann.text}\n`;
        break;
    }

    output += '\n';
  });

  output += `---\n`;
  return output;
}

// =============================================================================
// Diff Annotation Export
// =============================================================================

// exportDiffAnnotations formats diff annotations into structured markdown
// grouped by file path. Used when submitting code review feedback.
export function exportDiffAnnotations(
  annotations: DiffAnnotation[],
): string {
  if (annotations.length === 0) {
    return '# Code Review\n\nNo feedback provided.';
  }

  const grouped = new Map<string, DiffAnnotation[]>();
  for (const ann of annotations) {
    const existing = grouped.get(ann.filePath) || [];
    existing.push(ann);
    grouped.set(ann.filePath, existing);
  }

  let output = '# Code Review Feedback\n\n';

  for (const [filePath, fileAnnotations] of grouped) {
    output += `## ${filePath}\n\n`;

    const sorted = [...fileAnnotations].sort((a, b) => {
      const aScope = a.scope ?? 'line';
      const bScope = b.scope ?? 'line';
      if (aScope !== bScope) {
        return aScope === 'file' ? -1 : 1;
      }
      return a.lineStart - b.lineStart;
    });

    for (const ann of sorted) {
      const scope = ann.scope ?? 'line';

      if (scope === 'file') {
        output += `### File Comment\n`;
        if (ann.text) {
          output += `${ann.text}\n`;
        }
        if (ann.suggestedCode) {
          output += `\n**Suggested code:**\n\`\`\`\n${ann.suggestedCode}\n\`\`\`\n`;
        }
        output += '\n';
        continue;
      }

      const lineRange =
        ann.lineStart === ann.lineEnd
          ? `Line ${ann.lineStart}`
          : `Lines ${ann.lineStart}-${ann.lineEnd}`;

      output += `### ${lineRange} (${ann.side})\n`;

      if (ann.text) {
        output += `${ann.text}\n`;
      }

      if (ann.suggestedCode) {
        output += `\n**Suggested code:**\n\`\`\`\n${ann.suggestedCode}\n\`\`\`\n`;
      }

      output += '\n';
    }
  }

  return output;
}

// =============================================================================
// Agent Feedback Wrapper
// =============================================================================

// wrapFeedbackForAgent wraps feedback output with the deny preamble. This
// uses strong directive framing that was tuned in plannotator to ensure
// Claude respects the feedback and revises accordingly.
export function wrapFeedbackForAgent(
  feedback: string,
  toolName: string = 'ExitPlanMode',
): string {
  return (
    `YOUR PLAN WAS NOT APPROVED.\n\n` +
    `You MUST revise the plan to address ALL of the feedback below ` +
    `before calling ${toolName} again.\n\n` +
    `Rules:\n` +
    `- Do not resubmit the same plan unchanged.\n` +
    `- Do NOT change the plan title (first # heading) unless the user ` +
    `explicitly asks you to.\n\n` +
    `${feedback || 'Plan changes requested'}`
  );
}
