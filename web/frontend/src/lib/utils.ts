// Utility functions for the application.

import { clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

// Combine clsx and tailwind-merge for class name handling.
// Allows combining conditional classes and properly merging Tailwind utilities.
export function cn(...inputs: (string | undefined | null | false)[]): string {
  return twMerge(clsx(inputs));
}

// Agent display name interface.
// Uses `| undefined` to be compatible with exactOptionalPropertyTypes.
interface AgentLike {
  name: string;
  project_key?: string | undefined;
  git_branch?: string | undefined;
}

// Format an agent's display name with project and branch context.
// Returns format: "AgentName@project.git/branch" when project_key and git_branch are available.
// Falls back to just the name if context info is missing.
export function formatAgentDisplayName(agent: AgentLike): string {
  const { name, project_key, git_branch } = agent;

  // Guard against non-string values from the API.
  const projectKeyStr = typeof project_key === 'string' ? project_key : '';
  const gitBranchStr = typeof git_branch === 'string' ? git_branch : '';

  if (!projectKeyStr && !gitBranchStr) {
    return name;
  }

  // Extract the directory name from project_key (e.g., "subtrate-react-re-write" from path).
  const projectName = projectKeyStr
    ? projectKeyStr.split('/').pop() ?? projectKeyStr
    : '';

  if (projectName && gitBranchStr) {
    return `${name}@${projectName}.git/${gitBranchStr}`;
  } else if (projectName) {
    return `${name}@${projectName}`;
  } else if (gitBranchStr) {
    return `${name}/${gitBranchStr}`;
  }

  return name;
}

// Get short display name (just the agent name).
export function getAgentShortName(agent: AgentLike): string {
  return agent.name;
}

// Get the project context string (project.git/branch) without the agent name.
export function getAgentContext(agent: AgentLike): string | null {
  const { project_key, git_branch } = agent;

  // Guard against non-string values from the API.
  const projectKeyStr = typeof project_key === 'string' ? project_key : '';
  const gitBranchStr = typeof git_branch === 'string' ? git_branch : '';

  if (!projectKeyStr && !gitBranchStr) {
    return null;
  }

  // Extract the directory name from project_key (e.g., "subtrate-react-re-write" from path).
  const projectName = projectKeyStr
    ? projectKeyStr.split('/').pop() ?? projectKeyStr
    : '';

  if (projectName && gitBranchStr) {
    return `${projectName}.git/${gitBranchStr}`;
  } else if (projectName) {
    return projectName;
  } else if (gitBranchStr) {
    return gitBranchStr;
  }

  return null;
}
