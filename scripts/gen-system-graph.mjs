#!/usr/bin/env node
/**
 * Deterministic system-graph transpiler driven by docs/system-graph.yaml.
 * Usage:
 *   node scripts/gen-system-graph.mjs           # Write D2 views and docs/system-graph.md
 *   node scripts/gen-system-graph.mjs --render  # Also render SVGs via `d2` CLI
 *   node scripts/gen-system-graph.mjs --check   # Exit non-zero if generated files differ
 *   node scripts/gen-system-graph.mjs --json    # Print validated YAML as formatted JSON
 */
import { readFileSync, writeFileSync, existsSync, chmodSync } from 'node:fs';
import { resolve } from 'node:path';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';

const require = createRequire(resolve(import.meta.dirname, '../apps/web/package.json'));
const Ajv = require('ajv');
const { parse } = require('yaml');

const ROOT_DIR = resolve(import.meta.dirname, '..');
const GRAPH_YAML_PATH = resolve(ROOT_DIR, 'docs/system-graph.yaml');
const SCHEMA_PATH = resolve(ROOT_DIR, 'schema/system-graph.schema.json');
const OVERVIEW_D2_PATH = resolve(ROOT_DIR, 'docs/graph-overview.d2');
const DATAFLOW_D2_PATH = resolve(ROOT_DIR, 'docs/graph-dataflow.d2');
const OVERVIEW_SVG_PATH = resolve(ROOT_DIR, 'docs/graph-overview.svg');
const DATAFLOW_SVG_PATH = resolve(ROOT_DIR, 'docs/graph-dataflow.svg');
const MARKDOWN_PATH = resolve(ROOT_DIR, 'docs/system-graph.md');

const LAYER_META = {
  edge: { title: 'L1: Edge / Client', fill: '#1c1917', stroke: '#fb7185' },
  service: { title: 'L2: Runtime Service', fill: '#18181b', stroke: '#f59e0b' },
  worker: { title: 'L3: Async / Batch / CLI', fill: '#171717', stroke: '#38bdf8' },
  store: { title: 'L4: Data & State Stores', fill: '#141917', stroke: '#34d399' },
  external: { title: 'L5: External Systems', fill: '#171520', stroke: '#a78bfa' },
};

const STATUS_BADGE = {
  active: '',
  suspended: ' [SUSPENDED]',
  legacy_mirror: ' [LEGACY MIRROR]',
  manual: ' [MANUAL]',
};

const CONCURRENCY_STYLE = {
  none_put: { stroke: '#f43f5e', label: 'PUT (No Lock)', dash: 0 },
  generation_lock: { stroke: '#34d399', label: 'IfGen (Locked)', dash: 0 },
  shared_pvc_single: { stroke: '#f59e0b', label: 'PVC FS', dash: 0 },
  client_local: { stroke: '#a8a29e', label: 'LocalStorage', dash: 0 },
  read_only: { stroke: '#94a3b8', label: 'Read', dash: 4 },
};

function d2Str(value) {
  return JSON.stringify(String(value)).replace(/\$/g, '\\$');
}

function safeId(value) {
  return String(value).replace(/[^a-zA-Z0-9_]/g, '_');
}

function entryFilePath(entry) {
  return String(entry).split('#')[0];
}

/**
 * Validates parsed graph data against JSON Schema, entry paths, and graph invariants.
 */
export function validateSystemGraph(graph, rootDir = ROOT_DIR) {
  const schema = JSON.parse(readFileSync(resolve(rootDir, 'schema/system-graph.schema.json'), 'utf8'));
  const validate = new Ajv({ allErrors: true }).compile(schema);
  if (!validate(graph)) {
    throw new Error(`System graph schema violation: ${JSON.stringify(validate.errors)}`);
  }

  const hostMap = new Map();
  for (const hg of graph.host_groups) {
    if (hostMap.has(hg.id)) throw new Error(`Duplicate host_group id: ${hg.id}`);
    if (!existsSync(resolve(rootDir, entryFilePath(hg.entry)))) {
      throw new Error(`Missing host_group entry file: ${hg.entry}`);
    }
    hostMap.set(hg.id, hg);
  }
  for (const hg of graph.host_groups) {
    if (hg.parent && !hostMap.has(hg.parent)) {
      throw new Error(`Unknown host_group parent: ${hg.parent}`);
    }
  }

  const nodeMap = new Map();
  for (const node of graph.nodes) {
    if (nodeMap.has(node.id)) throw new Error(`Duplicate node id: ${node.id}`);
    if (node.host_group && !hostMap.has(node.host_group)) {
      throw new Error(`Unknown host_group on node ${node.id}: ${node.host_group}`);
    }
    if (!existsSync(resolve(rootDir, entryFilePath(node.entry)))) {
      throw new Error(`Missing node entry file for ${node.id}: ${node.entry}`);
    }
    nodeMap.set(node.id, node);
  }

  const edgeIds = new Set();
  for (const edge of graph.edges) {
    if (edgeIds.has(edge.id)) throw new Error(`Duplicate edge id: ${edge.id}`);
    edgeIds.add(edge.id);
    const src = nodeMap.get(edge.from);
    const dst = nodeMap.get(edge.to);
    if (!src) throw new Error(`Unknown from node in edge ${edge.id}: ${edge.from}`);
    if (!dst) throw new Error(`Unknown to node in edge ${edge.id}: ${edge.to}`);
    if (!existsSync(resolve(rootDir, entryFilePath(edge.entry)))) {
      throw new Error(`Missing edge entry file for ${edge.id}: ${edge.entry}`);
    }
    if (edge.kind === 'data') {
      if (src.layer === 'store' || dst.layer !== 'store') {
        throw new Error(`Data edge ${edge.id} must be Actor -> Store (got ${src.layer} -> ${dst.layer})`);
      }
      if ((edge.op === 'write' || edge.op === 'rw') && edge.concurrency === 'read_only') {
        throw new Error(`Write/RW data edge ${edge.id} cannot use concurrency: read_only`);
      }
    }
  }

  return graph;
}

function renderNodeBlock(lines, node, strokeColor, indent = '  ') {
  const nid = safeId(node.id);
  const badge = STATUS_BADGE[node.status] || '';
  const hostNote = node.host_group ? `\n[${node.host_group}]` : '';
  const text = `${node.label}${badge}${hostNote}\n${entryFilePath(node.entry)}`;
  const isMuted = node.status === 'suspended' || node.status === 'legacy_mirror';
  lines.push(`${indent}${nid}: ${d2Str(text)} {`);
  lines.push(`${indent}  shape: ${node.layer === 'store' ? 'cylinder' : 'rectangle'}`);
  lines.push(`${indent}  style.border-radius: 8`);
  lines.push(`${indent}  style.fill: ${isMuted ? '"#1c1917"' : '"#292524"'}`);
  lines.push(`${indent}  style.stroke: ${d2Str(isMuted ? '#78716c' : strokeColor)}`);
  lines.push(`${indent}  style.font-color: ${isMuted ? '"#a8a29e"' : '"#fafaf9"'}`);
  if (node.status === 'suspended') {
    lines.push(`${indent}  style.stroke-dash: 4`);
  }
  lines.push(`${indent}}`);
}

function edgeStyle(edge) {
  if (edge.status === 'suspended') {
    return { stroke: '#78716c', dash: 5, label: `${edge.kind} (suspended)` };
  }
  if (edge.kind === 'data') {
    const base = CONCURRENCY_STYLE[edge.concurrency] || CONCURRENCY_STYLE.read_only;
    const opTag = edge.op.toUpperCase();
    const multiTag = edge.multi_target ? ' [MULTI]' : '';
    return {
      stroke: base.stroke,
      dash: base.dash,
      label: `${opTag}: ${base.label}${multiTag}`,
    };
  }
  if (edge.kind === 'request') {
    return { stroke: '#38bdf8', dash: 0, label: `HTTP (${edge.transport})` };
  }
  return {
    stroke: '#a78bfa',
    dash: edge.status === 'manual' ? 3 : 0,
    label: `invoke${edge.status === 'manual' ? ' (manual)' : ''}`,
  };
}

/**
 * Generates L1-L5 layered overview diagram including host_group containment.
 */
export function buildOverviewD2(graph) {
  const lines = [
    '# Generated by scripts/gen-system-graph.mjs from docs/system-graph.yaml. Do not edit manually.',
    'direction: right',
    '',
  ];

  const grouped = new Map(Object.keys(LAYER_META).map((k) => [k, []]));
  for (const node of graph.nodes) {
    grouped.get(node.layer).push(node);
  }

  const d2Path = new Map();
  for (const [layerKey, nodes] of grouped.entries()) {
    if (nodes.length === 0) continue;
    const meta = LAYER_META[layerKey];
    lines.push(`${layerKey}: ${d2Str(meta.title)} {`);
    lines.push(`  style.fill: ${d2Str(meta.fill)}`);
    lines.push(`  style.stroke: ${d2Str(meta.stroke)}`);
    lines.push('  style.border-radius: 12');

    if (layerKey === 'service') {
      const inProcess = nodes.filter((n) => n.host_group === 'monitor_process');
      const outside = nodes.filter((n) => n.host_group !== 'monitor_process');
      if (inProcess.length > 0) {
        const hg = graph.host_groups.find((h) => h.id === 'monitor_process');
        lines.push(`  monitor_process: ${d2Str(hg ? hg.label : 'monitor_process')} {`);
        lines.push('    style.fill: "#0c0a09"');
        lines.push('    style.stroke: "#fbbf24"');
        lines.push('    style.stroke-dash: 4');
        lines.push('    style.border-radius: 10');
        for (const node of inProcess) {
          renderNodeBlock(lines, node, meta.stroke, '    ');
          d2Path.set(node.id, `${layerKey}.monitor_process.${safeId(node.id)}`);
        }
        lines.push('  }');
      }
      for (const node of outside) {
        renderNodeBlock(lines, node, meta.stroke, '  ');
        d2Path.set(node.id, `${layerKey}.${safeId(node.id)}`);
      }
    } else {
      for (const node of nodes) {
        renderNodeBlock(lines, node, meta.stroke, '  ');
        d2Path.set(node.id, `${layerKey}.${safeId(node.id)}`);
      }
    }
    lines.push('}');
    lines.push('');
  }

  lines.push('# Directed Edges');
  for (const edge of graph.edges) {
    const src = d2Path.get(edge.from);
    const dst = d2Path.get(edge.to);
    if (!src || !dst) continue;
    const st = edgeStyle(edge);
    lines.push(`${src} -> ${dst}: ${d2Str(st.label)} {`);
    lines.push(`  style.stroke: ${d2Str(st.stroke)}`);
    if (st.dash > 0) {
      lines.push(`  style.stroke-dash: ${st.dash}`);
    }
    lines.push('}');
  }
  lines.push('');
  return lines.join('\n');
}

/**
 * Generates untangled Data & Concurrency view (only kind=data edges, Actor -> Store).
 */
export function buildDataflowD2(graph) {
  const lines = [
    '# Generated by scripts/gen-system-graph.mjs from docs/system-graph.yaml. Do not edit manually.',
    'direction: right',
    '',
  ];

  const dataEdges = graph.edges.filter((e) => e.kind === 'data');
  const actorIds = new Set(dataEdges.map((e) => e.from));
  const storeIds = new Set(dataEdges.map((e) => e.to));
  const actors = graph.nodes.filter((n) => actorIds.has(n.id));
  const stores = graph.nodes.filter((n) => storeIds.has(n.id));

  lines.push('actors: "Data Readers & Writers (L1 - L3)" {');
  lines.push('  style.fill: "#18181b"');
  lines.push('  style.stroke: "#f59e0b"');
  lines.push('  style.border-radius: 12');
  for (const node of actors) {
    renderNodeBlock(lines, node, LAYER_META[node.layer].stroke, '  ');
  }
  lines.push('}');
  lines.push('');

  lines.push('stores: "Data & State Stores (L4)" {');
  lines.push('  style.fill: "#141917"');
  lines.push('  style.stroke: "#34d399"');
  lines.push('  style.border-radius: 12');
  for (const node of stores) {
    renderNodeBlock(lines, node, LAYER_META.store.stroke, '  ');
  }
  lines.push('}');
  lines.push('');

  lines.push('# Store Access Edges (Actor -> Store)');
  for (const edge of dataEdges) {
    const st = edgeStyle(edge);
    const targetsLabel = edge.targets.length <= 2 ? edge.targets.join(', ') : `${edge.targets[0]} (+${edge.targets.length - 1})`;
    const label = `${st.label}\n${targetsLabel}`;
    lines.push(`actors.${safeId(edge.from)} -> stores.${safeId(edge.to)}: ${d2Str(label)} {`);
    lines.push(`  style.stroke: ${d2Str(st.stroke)}`);
    if (st.dash > 0) {
      lines.push(`  style.stroke-dash: ${st.dash}`);
    }
    if (edge.concurrency === 'none_put') {
      lines.push('  style.stroke-width: 2');
    }
    lines.push('}');
  }
  lines.push('');
  return lines.join('\n');
}

/**
 * Generates Markdown architecture reference + Store Writer matrix from YAML.
 */
export function buildSystemGraphMarkdown(graph) {
  const nodeMap = new Map(graph.nodes.map((n) => [n.id, n]));
  const writeEdges = graph.edges.filter((e) => e.kind === 'data' && (e.op === 'write' || e.op === 'rw'));

  const lines = [
    '# システム構成・データ経路リファレンス',
    '',
    '本ドキュメントは [system-graph.yaml](system-graph.yaml)（記述規約: [system-graph-rules.md](system-graph-rules.md)）から自動生成されています。手動編集せず `node scripts/gen-system-graph.mjs --render` を実行してください。',
    '',
    '## 1. 全体構成ビュー (L1 - L5)',
    '',
    '![System Overview](graph-overview.svg)',
    '',
    '## 2. データ読み書き・排他制御ビュー (`kind: data`)',
    '',
    '![Data Concurrency Flow](graph-dataflow.svg)',
    '',
    '## 3. 書き込み経路マトリクス（Store 逆引き）',
    '',
    '| Store | Actor | Op | Concurrency | Multi-Target | Status | Entry | Targets |',
    '|---|---|---|---|---|---|---|---|',
  ];

  for (const e of writeEdges) {
    const store = nodeMap.get(e.to);
    const actor = nodeMap.get(e.from);
    lines.push(
      `| \`${store.id}\` (${store.label}) | \`${actor.id}\` (${actor.label}) | \`${e.op}\` | \`${e.concurrency}\` | ${e.multi_target ? 'yes' : 'no'} | \`${e.status}\` | \`${e.entry}\` | ${e.targets.map((t) => `\`${t}\``).join(', ')} |`
    );
  }

  lines.push('');
  lines.push('## 4. 同居グループ (`host_groups`)');
  lines.push('');
  lines.push('| ID | Label | Parent | Entry | Notes |');
  lines.push('|---|---|---|---|---|');
  for (const hg of graph.host_groups) {
    lines.push(`| \`${hg.id}\` | ${hg.label} | ${hg.parent ? `\`${hg.parent}\`` : '-'} | \`${hg.entry}\` | ${hg.notes || ''} |`);
  }
  lines.push('');
  return lines.join('\n');
}

function main() {
  const args = new Set(process.argv.slice(2));
  const rawYaml = readFileSync(GRAPH_YAML_PATH, 'utf8');
  const graph = validateSystemGraph(parse(rawYaml), ROOT_DIR);

  if (args.has('--json')) {
    process.stdout.write(`${JSON.stringify(graph, null, 2)}\n`);
    return;
  }

  const overviewD2 = buildOverviewD2(graph);
  const dataflowD2 = buildDataflowD2(graph);
  const markdown = buildSystemGraphMarkdown(graph);

  if (args.has('--check')) {
    const currOverview = existsSync(OVERVIEW_D2_PATH) ? readFileSync(OVERVIEW_D2_PATH, 'utf8') : '';
    const currDataflow = existsSync(DATAFLOW_D2_PATH) ? readFileSync(DATAFLOW_D2_PATH, 'utf8') : '';
    const currMd = existsSync(MARKDOWN_PATH) ? readFileSync(MARKDOWN_PATH, 'utf8') : '';
    if (currOverview !== overviewD2 || currDataflow !== dataflowD2 || currMd !== markdown) {
      console.error('System graph artifacts are out of date. Run: node scripts/gen-system-graph.mjs --render');
      process.exit(1);
    }
    console.log('System graph D2/Markdown artifacts match docs/system-graph.yaml.');
    return;
  }

  writeFileSync(OVERVIEW_D2_PATH, overviewD2, 'utf8');
  writeFileSync(DATAFLOW_D2_PATH, dataflowD2, 'utf8');
  writeFileSync(MARKDOWN_PATH, markdown, 'utf8');
  console.log('Generated docs/graph-overview.d2, docs/graph-dataflow.d2, and docs/system-graph.md');

  if (args.has('--render')) {
    execFileSync('d2', ['--theme=200', '--pad=28', OVERVIEW_D2_PATH, OVERVIEW_SVG_PATH], { stdio: 'inherit' });
    execFileSync('d2', ['--theme=200', '--pad=28', DATAFLOW_D2_PATH, DATAFLOW_SVG_PATH], { stdio: 'inherit' });
    chmodSync(OVERVIEW_SVG_PATH, 0o644);
    chmodSync(DATAFLOW_SVG_PATH, 0o644);
    console.log('Rendered docs/graph-overview.svg and docs/graph-dataflow.svg');
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main();
}
