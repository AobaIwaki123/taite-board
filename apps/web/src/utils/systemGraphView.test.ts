import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { parse } from 'yaml';
import {
  buildSvgEdgeLookup,
  computeVisibility,
  decodeD2ClassTarget,
  type FilterState,
  type SystemGraphData,
} from './systemGraphView.ts';

const root = resolve(import.meta.dirname, '../../../..');
const graph = parse(readFileSync(resolve(root, 'docs/system-graph.yaml'), 'utf8')) as SystemGraphData;
const knownIds = new Set(graph.nodes.map((n) => n.id));

function b64(text: string): string {
  return Buffer.from(text, 'utf8').toString('base64');
}

function defaultFilters(): FilterState {
  return {
    layers: new Set(['edge', 'service', 'worker', 'store', 'external']),
    edgeKinds: new Set(['request', 'data', 'invoke']),
    hideSuspended: false,
    hideLegacyMirrors: false,
    hideManual: false,
    concurrencyFilter: 'all',
    searchQuery: '',
    hiddenNodeIds: new Set(),
  };
}

test('decodeD2ClassTarget parses cross-container, shared-container, and node classes', () => {
  const sharedEdge = decodeD2ClassTarget(b64('edge.(web_spa -&gt; cloudflare_ingress)[0]'), knownIds);
  assert.deepEqual(sharedEdge, { kind: 'edge', from: 'web_spa', to: 'cloudflare_ingress', index: 0 });

  const multiEdge = decodeD2ClassTarget(
    b64('(service.monitor_process.monitor_api -&gt; store.gcs_data_groups)[1]'),
    knownIds
  );
  assert.deepEqual(multiEdge, { kind: 'edge', from: 'monitor_api', to: 'gcs_data_groups', index: 1 });

  const node = decodeD2ClassTarget(`shape ${b64('actors.monitor_api')}`, knownIds);
  assert.deepEqual(node, { kind: 'node', nodeId: 'monitor_api' });

  const containerOnly = decodeD2ClassTarget(b64('service.monitor_process'), knownIds);
  assert.equal(containerOnly, null);
});

test('buildSvgEdgeLookup differentiates multi-edges by emission index', () => {
  const overviewLookup = buildSvgEdgeLookup(graph.edges, 'overview');
  const first = overviewLookup.get('monitor_api->gcs_data_groups#0');
  const second = overviewLookup.get('monitor_api->gcs_data_groups#1');
  assert.equal(first?.op, 'read');
  assert.equal(second?.op, 'write');
  assert.equal(second?.concurrency, 'none_put');

  const dataflowLookup = buildSvgEdgeLookup(graph.edges, 'dataflow');
  assert.equal(dataflowLookup.has('web_spa->cloudflare_ingress#0'), false);
  assert.equal(dataflowLookup.get('timetree_sync->gcs_data_groups#0')?.concurrency, 'generation_lock');
});

test('computeVisibility filters suspended/legacy nodes, per-node hides, and 1-hop selection', () => {
  const filters = defaultFilters();
  filters.hideSuspended = true;
  filters.hideLegacyMirrors = true;
  filters.hiddenNodeIds.add('discord_webhook');

  const res = computeVisibility(graph, filters, 'timetree_sync', 'overview');
  assert.equal(res.visibleNodeIds.has('crawler_cron'), false);
  assert.equal(res.visibleNodeIds.has('gcs_data_mirrors'), false);
  assert.equal(res.visibleNodeIds.has('discord_webhook'), false);

  assert.equal(res.highlightedNodeIds.has('timetree_sync'), true);
  assert.equal(res.highlightedNodeIds.has('gcs_data_groups'), true);
  assert.equal(res.highlightedNodeIds.has('lumitree_api'), true);
  assert.equal(res.dimmedNodeIds.has('web_spa'), true);
});
