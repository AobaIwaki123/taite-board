import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { execFileSync } from 'node:child_process';
import { parse } from 'yaml';
import { validateSystemGraph } from '../../../../scripts/gen-system-graph.mjs';

const root = resolve(import.meta.dirname, '../../../..');
const rawYaml = readFileSync(resolve(root, 'docs/system-graph.yaml'), 'utf8');
const graph = parse(rawYaml);

test('system-graph.yaml passes schema and referential checks with no orphan nodes', () => {
  validateSystemGraph(graph, root);
  const connected = new Set<string>();
  for (const edge of graph.edges) {
    connected.add(edge.from);
    connected.add(edge.to);
  }
  const orphans = graph.nodes
    .filter((node: { id: string }) => !connected.has(node.id))
    .map((node: { id: string }) => node.id);
  assert.deepEqual(orphans, [], `unexpected orphan nodes: ${orphans.join(', ')}`);
});

test('generated D2 and Markdown system graph artifacts stay in sync', () => {
  execFileSync(process.execPath, [resolve(root, 'scripts/gen-system-graph.mjs'), '--check'], { stdio: 'pipe' });
  assert.ok(existsSync(resolve(root, 'docs/graph-overview.svg')), 'missing docs/graph-overview.svg');
  assert.ok(existsSync(resolve(root, 'docs/graph-dataflow.svg')), 'missing docs/graph-dataflow.svg');
});

test('a reversed Store -> Actor data edge is rejected', () => {
  const broken = structuredClone(graph);
  const dataEdge = broken.edges.find((e: { kind: string }) => e.kind === 'data');
  const originalFrom = dataEdge.from;
  dataEdge.from = dataEdge.to;
  dataEdge.to = originalFrom;
  assert.throws(() => validateSystemGraph(broken, root), /must be Actor -> Store/);
});

test('a write data edge marked read_only is rejected', () => {
  const broken = structuredClone(graph);
  const writeEdge = broken.edges.find((e: { kind: string; op?: string }) => e.kind === 'data' && (e.op === 'write' || e.op === 'rw'));
  writeEdge.concurrency = 'read_only';
  assert.throws(() => validateSystemGraph(broken, root), /cannot use concurrency: read_only/);
});
