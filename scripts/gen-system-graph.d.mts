export function validateSystemGraph(graph: unknown, rootDir?: string): Record<string, unknown>;
export function buildOverviewD2(graph: Record<string, unknown>): string;
export function buildDataflowD2(graph: Record<string, unknown>): string;
export function buildSystemGraphMarkdown(graph: Record<string, unknown>): string;
