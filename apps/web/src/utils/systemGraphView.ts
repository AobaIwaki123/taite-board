export type LayerId = 'edge' | 'service' | 'worker' | 'store' | 'external';
export type GraphStatus = 'active' | 'suspended' | 'legacy_mirror' | 'manual';
export type EdgeKind = 'request' | 'data' | 'invoke';
export type DataOp = 'read' | 'write' | 'rw';
export type ConcurrencyMode =
  | 'none_put'
  | 'generation_lock'
  | 'shared_pvc_single'
  | 'client_local'
  | 'read_only';
export type GraphViewMode = 'board' | 'overview' | 'dataflow';
export type ConcurrencyFilter = 'all' | 'writes_only' | 'none_put' | 'generation_lock';
export type HiddenRenderMode = 'hide' | 'dim';

export interface GraphHostGroup {
  id: string;
  label: string;
  parent?: string;
  entry: string;
  notes?: string;
}

export interface GraphNode {
  id: string;
  layer: LayerId;
  kind: string;
  label: string;
  status: GraphStatus;
  host_group?: string;
  entry: string;
  notes?: string;
}

export interface GraphEdge {
  id: string;
  from: string;
  to: string;
  kind: EdgeKind;
  op?: DataOp;
  transport: string;
  concurrency?: ConcurrencyMode;
  multi_target?: boolean;
  status: GraphStatus;
  targets?: string[];
  entry: string;
  notes?: string;
}

export interface SystemGraphData {
  schema_version: number;
  host_groups: GraphHostGroup[];
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export type SvgElementTarget =
  | { kind: 'node'; nodeId: string }
  | { kind: 'edge'; from: string; to: string; index: number };

export interface FilterState {
  layers: Set<LayerId>;
  edgeKinds: Set<EdgeKind>;
  hideSuspended: boolean;
  hideLegacyMirrors: boolean;
  hideManual: boolean;
  concurrencyFilter: ConcurrencyFilter;
  searchQuery: string;
  hiddenNodeIds: Set<string>;
}

export interface VisibilityResult {
  viewNodeIds: Set<string>;
  visibleNodeIds: Set<string>;
  highlightedNodeIds: Set<string>;
  dimmedNodeIds: Set<string>;
  visibleEdgeIds: Set<string>;
  highlightedEdgeIds: Set<string>;
  dimmedEdgeIds: Set<string>;
}

function decodeBase64Utf8(token: string): string | null {
  if (!/^[A-Za-z0-9+/=]{4,}$/.test(token)) return null;
  try {
    if (typeof atob === 'function') {
      const bin = atob(token);
      const bytes = Uint8Array.from(bin, (ch) => ch.charCodeAt(0));
      return new TextDecoder().decode(bytes);
    }
    return Buffer.from(token, 'base64').toString('utf8');
  } catch {
    return null;
  }
}

/**
 * Decodes D2 SVG `<g>` base64 class names into either a graph node or directed edge reference.
 * Handles both cross-container `(src -&gt; dst)[i]` and shared-container `prefix.(a -&gt; b)[i]`.
 */
export function decodeD2ClassTarget(rawClass: string, knownNodeIds: Set<string>): SvgElementTarget | null {
  const tokens = rawClass.trim().split(/\s+/);
  for (const token of tokens) {
    const decoded = decodeBase64Utf8(token);
    if (!decoded) continue;

    const edgeMatch = decoded.match(/\(([^()]+?)\s*(?:-&gt;|->)\s*([^()]+?)\)\[(\d+)\]$/);
    if (edgeMatch) {
      const from = edgeMatch[1].trim().split('.').pop() || '';
      const to = edgeMatch[2].trim().split('.').pop() || '';
      const index = Number.parseInt(edgeMatch[3], 10);
      if (knownNodeIds.has(from) && knownNodeIds.has(to) && Number.isFinite(index)) {
        return { kind: 'edge', from, to, index };
      }
      continue;
    }

    const leaf = decoded.trim().split('.').pop() || '';
    if (knownNodeIds.has(leaf)) {
      return { kind: 'node', nodeId: leaf };
    }
  }
  return null;
}

/**
 * Maps D2 edge occurrence keys (`from->to#idx`) to GraphEdge objects in emission order.
 */
export function buildSvgEdgeLookup(edges: GraphEdge[], viewMode: GraphViewMode): Map<string, GraphEdge> {
  const relevant = viewMode === 'dataflow' ? edges.filter((e) => e.kind === 'data') : edges;
  const counts = new Map<string, number>();
  const lookup = new Map<string, GraphEdge>();

  for (const edge of relevant) {
    const pairKey = `${edge.from}->${edge.to}`;
    const idx = counts.get(pairKey) ?? 0;
    counts.set(pairKey, idx + 1);
    lookup.set(`${pairKey}#${idx}`, edge);
  }
  return lookup;
}

function matchesSearch(node: GraphNode, query: string, connectedEdges: GraphEdge[]): boolean {
  if (!query) return true;
  const q = query.toLowerCase();
  if (
    node.id.toLowerCase().includes(q) ||
    node.label.toLowerCase().includes(q) ||
    node.entry.toLowerCase().includes(q) ||
    (node.notes && node.notes.toLowerCase().includes(q))
  ) {
    return true;
  }
  return connectedEdges.some(
    (e) =>
      e.entry.toLowerCase().includes(q) ||
      (e.targets && e.targets.some((t) => t.toLowerCase().includes(q)))
  );
}

function edgeMatchesConcurrency(edge: GraphEdge, mode: ConcurrencyFilter): boolean {
  if (mode === 'all') return true;
  if (edge.kind !== 'data') return false;
  if (mode === 'writes_only') return edge.op === 'write' || edge.op === 'rw';
  return edge.concurrency === mode;
}

/**
 * Computes visible, highlighted (1-hop neighborhood), and dimmed nodes/edges.
 */
export function computeVisibility(
  graph: SystemGraphData,
  filters: FilterState,
  selectedNodeId: string | null,
  viewMode: GraphViewMode
): VisibilityResult {
  const viewEdges = viewMode === 'dataflow' ? graph.edges.filter((e) => e.kind === 'data') : graph.edges;
  const viewNodeIds = new Set<string>();
  if (viewMode === 'dataflow') {
    for (const e of viewEdges) {
      viewNodeIds.add(e.from);
      viewNodeIds.add(e.to);
    }
  } else {
    for (const n of graph.nodes) {
      viewNodeIds.add(n.id);
    }
  }

  const edgesByNode = new Map<string, GraphEdge[]>();
  for (const e of viewEdges) {
    if (!edgesByNode.has(e.from)) edgesByNode.set(e.from, []);
    if (!edgesByNode.has(e.to)) edgesByNode.set(e.to, []);
    edgesByNode.get(e.from)!.push(e);
    edgesByNode.get(e.to)!.push(e);
  }

  const q = filters.searchQuery.trim();
  const visibleNodeIds = new Set<string>();
  for (const node of graph.nodes) {
    if (!viewNodeIds.has(node.id)) continue;
    if (!filters.layers.has(node.layer)) continue;
    if (filters.hideSuspended && node.status === 'suspended') continue;
    if (filters.hideLegacyMirrors && node.status === 'legacy_mirror') continue;
    if (filters.hideManual && node.status === 'manual') continue;
    if (filters.hiddenNodeIds.has(node.id)) continue;
    if (!matchesSearch(node, q, edgesByNode.get(node.id) ?? [])) continue;
    visibleNodeIds.add(node.id);
  }

  const visibleEdgeIds = new Set<string>();
  for (const edge of viewEdges) {
    if (!visibleNodeIds.has(edge.from) || !visibleNodeIds.has(edge.to)) continue;
    if (!filters.edgeKinds.has(edge.kind)) continue;
    if (filters.hideSuspended && edge.status === 'suspended') continue;
    if (filters.hideLegacyMirrors && edge.status === 'legacy_mirror') continue;
    if (filters.hideManual && edge.status === 'manual') continue;
    if (!edgeMatchesConcurrency(edge, filters.concurrencyFilter)) continue;
    visibleEdgeIds.add(edge.id);
  }

  const highlightedNodeIds = new Set<string>();
  const dimmedNodeIds = new Set<string>();
  const highlightedEdgeIds = new Set<string>();
  const dimmedEdgeIds = new Set<string>();

  if (selectedNodeId && visibleNodeIds.has(selectedNodeId)) {
    highlightedNodeIds.add(selectedNodeId);
    for (const edge of viewEdges) {
      if (!visibleEdgeIds.has(edge.id)) continue;
      if (edge.from === selectedNodeId || edge.to === selectedNodeId) {
        highlightedEdgeIds.add(edge.id);
        highlightedNodeIds.add(edge.from);
        highlightedNodeIds.add(edge.to);
      } else {
        dimmedEdgeIds.add(edge.id);
      }
    }
    for (const nodeId of visibleNodeIds) {
      if (!highlightedNodeIds.has(nodeId)) {
        dimmedNodeIds.add(nodeId);
      }
    }
  } else {
    for (const nodeId of visibleNodeIds) highlightedNodeIds.add(nodeId);
    for (const edgeId of visibleEdgeIds) highlightedEdgeIds.add(edgeId);
  }

  return {
    viewNodeIds,
    visibleNodeIds,
    highlightedNodeIds,
    dimmedNodeIds,
    visibleEdgeIds,
    highlightedEdgeIds,
    dimmedEdgeIds,
  };
}
