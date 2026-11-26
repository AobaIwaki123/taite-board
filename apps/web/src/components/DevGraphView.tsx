import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks';
import {
  buildSvgEdgeLookup,
  computeVisibility,
  decodeD2ClassTarget,
  type ConcurrencyFilter,
  type EdgeKind,
  type GraphEdge,
  type GraphNode,
  type GraphViewMode,
  type HiddenRenderMode,
  type LayerId,
  type SystemGraphData,
} from '../utils/systemGraphView';

const ALL_LAYERS: { id: LayerId; label: string }[] = [
  { id: 'edge', label: 'L1 Edge' },
  { id: 'service', label: 'L2 Service' },
  { id: 'worker', label: 'L3 Worker' },
  { id: 'store', label: 'L4 Store' },
  { id: 'external', label: 'L5 External' },
];

const ALL_EDGE_KINDS: { id: EdgeKind; label: string }[] = [
  { id: 'request', label: 'HTTP Request' },
  { id: 'data', label: 'Data Access' },
  { id: 'invoke', label: 'Invoke / External' },
];

interface ParsedSvgBindings {
  nodeElements: Map<string, SVGGElement[]>;
  edgeElements: Array<{ el: SVGGElement; edge: GraphEdge }>;
  viewWidth: number;
  viewHeight: number;
}

export function DevGraphView() {
  const [graph, setGraph] = useState<SystemGraphData | null>(null);
  const [viewMode, setViewMode] = useState<GraphViewMode>('overview');
  const [svgMarkup, setSvgMarkup] = useState<string>('');
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [renderingD2, setRenderingD2] = useState(false);

  const [layers, setLayers] = useState<Set<LayerId>>(() => new Set(['edge', 'service', 'worker', 'store', 'external']));
  const [edgeKinds, setEdgeKinds] = useState<Set<EdgeKind>>(() => new Set(['request', 'data', 'invoke']));
  const [hideSuspended, setHideSuspended] = useState(false);
  const [hideLegacyMirrors, setHideLegacyMirrors] = useState(false);
  const [hideManual, setHideManual] = useState(false);
  const [concurrencyFilter, setConcurrencyFilter] = useState<ConcurrencyFilter>('all');
  const [hiddenMode, setHiddenMode] = useState<HiddenRenderMode>('hide');
  const [searchQuery, setSearchQuery] = useState('');
  const [hiddenNodeIds, setHiddenNodeIds] = useState<Set<string>>(() => new Set());
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);

  const [transform, setTransform] = useState({ x: 24, y: 24, scale: 0.45 });
  const viewportRef = useRef<HTMLDivElement>(null);
  const svgHostRef = useRef<HTMLDivElement>(null);
  const bindingsRef = useRef<ParsedSvgBindings | null>(null);
  const dragStateRef = useRef<{ active: boolean; moved: boolean; startX: number; startY: number; origX: number; origY: number }>({
    active: false,
    moved: false,
    startX: 0,
    startY: 0,
    origX: 0,
    origY: 0,
  });

  const loadGraphData = useCallback(async (mode: GraphViewMode) => {
    try {
      const svgPath = mode === 'overview' ? '/__dev/graph-overview.svg' : '/__dev/graph-dataflow.svg';
      const [yamlRes, svgRes] = await Promise.all([
        fetch('/__dev/system-graph.json', { cache: 'no-store' }),
        fetch(svgPath, { cache: 'no-store' }),
      ]);
      if (!yamlRes.ok) throw new Error(`system-graph.json HTTP ${yamlRes.status}`);
      if (!svgRes.ok) throw new Error(`${svgPath} HTTP ${svgRes.status}`);
      const data = (await yamlRes.json()) as SystemGraphData;
      const svgText = await svgRes.text();
      setGraph(data);
      setSvgMarkup(svgText);
      setErrorMsg(null);
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : String(err));
    }
  }, []);

  useEffect(() => {
    void loadGraphData(viewMode);
  }, [loadGraphData, viewMode]);

  useEffect(() => {
    if (!import.meta.hot) return;
    const handler = () => {
      void loadGraphData(viewMode);
    };
    import.meta.hot.on('system-graph-updated', handler);
    return () => {
      import.meta.hot?.off('system-graph-updated', handler);
    };
  }, [loadGraphData, viewMode]);

  const handleRerenderD2 = async () => {
    setRenderingD2(true);
    try {
      const res = await fetch('/__dev/render-graph', { method: 'POST' });
      if (!res.ok) throw new Error(`render-graph HTTP ${res.status}`);
      await loadGraphData(viewMode);
    } catch (err) {
      setErrorMsg(err instanceof Error ? err.message : String(err));
    } finally {
      setRenderingD2(false);
    }
  };

  const fitToViewport = useCallback((viewWidth: number, viewHeight: number) => {
    const vp = viewportRef.current;
    if (!vp || viewWidth <= 0 || viewHeight <= 0) return;
    const vw = vp.clientWidth;
    const vh = vp.clientHeight;
    const scale = Math.max(0.1, Math.min(1.5, Math.min(vw / viewWidth, vh / viewHeight) * 0.94));
    const x = (vw - viewWidth * scale) / 2;
    const y = (vh - viewHeight * scale) / 2;
    setTransform({ x, y, scale });
  }, []);

  // Inject SVG markup imperatively so DOM filter updates never reset pan/zoom state.
  useEffect(() => {
    const host = svgHostRef.current;
    if (!host || !svgMarkup || !graph) return;

    host.innerHTML = svgMarkup;
    const outerSvg = host.querySelector('svg');
    if (!outerSvg) return;

    const vb = outerSvg.viewBox.baseVal;
    const viewWidth = vb && vb.width > 0 ? vb.width : Number(outerSvg.getAttribute('width')) || 2400;
    const viewHeight = vb && vb.height > 0 ? vb.height : Number(outerSvg.getAttribute('height')) || 1600;
    outerSvg.setAttribute('width', String(viewWidth));
    outerSvg.setAttribute('height', String(viewHeight));
    outerSvg.style.display = 'block';

    const knownNodeIds = new Set(graph.nodes.map((n) => n.id));
    const edgeLookup = buildSvgEdgeLookup(graph.edges, viewMode);
    const nodeElements = new Map<string, SVGGElement[]>();
    const edgeElements: Array<{ el: SVGGElement; edge: GraphEdge }> = [];

    const groups = outerSvg.querySelectorAll('g');
    const cleanups: Array<() => void> = [];

    for (const g of groups) {
      const rawClass = g.getAttribute('class') || '';
      const target = decodeD2ClassTarget(rawClass, knownNodeIds);
      if (!target) continue;

      if (target.kind === 'node') {
        if (!nodeElements.has(target.nodeId)) nodeElements.set(target.nodeId, []);
        nodeElements.get(target.nodeId)!.push(g);
        g.style.cursor = 'pointer';
        const onClick = (ev: MouseEvent) => {
          if (dragStateRef.current.moved) return;
          ev.stopPropagation();
          setSelectedNodeId((prev) => (prev === target.nodeId ? null : target.nodeId));
        };
        g.addEventListener('click', onClick);
        cleanups.push(() => g.removeEventListener('click', onClick));
      } else {
        const edge = edgeLookup.get(`${target.from}->${target.to}#${target.index}`);
        if (edge) {
          edgeElements.push({ el: g, edge });
        }
      }
    }

    bindingsRef.current = { nodeElements, edgeElements, viewWidth, viewHeight };
    fitToViewport(viewWidth, viewHeight);

    return () => {
      for (const fn of cleanups) fn();
    };
  }, [svgMarkup, graph, viewMode, fitToViewport]);

  const visibility = useMemo(() => {
    if (!graph) return null;
    return computeVisibility(
      graph,
      {
        layers,
        edgeKinds,
        hideSuspended,
        hideLegacyMirrors,
        hideManual,
        concurrencyFilter,
        searchQuery,
        hiddenNodeIds,
      },
      selectedNodeId,
      viewMode
    );
  }, [
    graph,
    layers,
    edgeKinds,
    hideSuspended,
    hideLegacyMirrors,
    hideManual,
    concurrencyFilter,
    searchQuery,
    hiddenNodeIds,
    selectedNodeId,
    viewMode,
  ]);

  useEffect(() => {
    const bindings = bindingsRef.current;
    if (!bindings || !visibility) return;

    for (const [nodeId, els] of bindings.nodeElements.entries()) {
      const isVisible = visibility.visibleNodeIds.has(nodeId);
      const isDimmed = isVisible && visibility.dimmedNodeIds.has(nodeId);
      for (const el of els) {
        if (!isVisible) {
          el.style.display = hiddenMode === 'hide' ? 'none' : '';
          el.style.opacity = hiddenMode === 'dim' ? '0.08' : '1';
          el.style.pointerEvents = hiddenMode === 'dim' ? 'none' : '';
        } else {
          el.style.display = '';
          el.style.opacity = isDimmed ? '0.16' : '1';
          el.style.pointerEvents = '';
        }
      }
    }

    for (const { el, edge } of bindings.edgeElements) {
      const isVisible = visibility.visibleEdgeIds.has(edge.id);
      const isDimmed = isVisible && visibility.dimmedEdgeIds.has(edge.id);
      if (!isVisible) {
        el.style.display = hiddenMode === 'hide' ? 'none' : '';
        el.style.opacity = hiddenMode === 'dim' ? '0.06' : '1';
      } else {
        el.style.display = '';
        el.style.opacity = isDimmed ? '0.12' : '1';
      }
    }
  }, [visibility, hiddenMode, svgMarkup]);

  const toggleLayer = (layer: LayerId) => {
    setLayers((prev) => {
      const next = new Set(prev);
      if (next.has(layer)) next.delete(layer);
      else next.add(layer);
      return next;
    });
  };

  const toggleEdgeKind = (kind: EdgeKind) => {
    setEdgeKinds((prev) => {
      const next = new Set(prev);
      if (next.has(kind)) next.delete(kind);
      else next.add(kind);
      return next;
    });
  };

  const toggleNodeHidden = (nodeId: string) => {
    setHiddenNodeIds((prev) => {
      const next = new Set(prev);
      if (next.has(nodeId)) next.delete(nodeId);
      else next.add(nodeId);
      return next;
    });
  };

  const handleWheel = (e: WheelEvent) => {
    e.preventDefault();
    const vp = viewportRef.current;
    if (!vp) return;
    const rect = vp.getBoundingClientRect();
    const cx = e.clientX - rect.left;
    const cy = e.clientY - rect.top;
    const factor = e.deltaY < 0 ? 1.14 : 1 / 1.14;

    setTransform((prev) => {
      const nextScale = Math.max(0.1, Math.min(4.5, prev.scale * factor));
      const ratio = nextScale / prev.scale;
      return {
        scale: nextScale,
        x: cx - (cx - prev.x) * ratio,
        y: cy - (cy - prev.y) * ratio,
      };
    });
  };

  const zoomCenter = (factor: number) => {
    const vp = viewportRef.current;
    if (!vp) return;
    const cx = vp.clientWidth / 2;
    const cy = vp.clientHeight / 2;
    setTransform((prev) => {
      const nextScale = Math.max(0.1, Math.min(4.5, prev.scale * factor));
      const ratio = nextScale / prev.scale;
      return {
        scale: nextScale,
        x: cx - (cx - prev.x) * ratio,
        y: cy - (cy - prev.y) * ratio,
      };
    });
  };

  const handlePointerDown = (e: PointerEvent) => {
    if (e.button !== 0) return;
    dragStateRef.current = {
      active: true,
      moved: false,
      startX: e.clientX,
      startY: e.clientY,
      origX: transform.x,
      origY: transform.y,
    };
  };

  const handlePointerMove = (e: PointerEvent) => {
    const d = dragStateRef.current;
    if (!d.active) return;
    const dx = e.clientX - d.startX;
    const dy = e.clientY - d.startY;
    if (Math.abs(dx) > 4 || Math.abs(dy) > 4) {
      d.moved = true;
    }
    setTransform((prev) => ({
      ...prev,
      x: d.origX + dx,
      y: d.origY + dy,
    }));
  };

  const handlePointerUp = () => {
    dragStateRef.current.active = false;
  };

  const selectedNode: GraphNode | null = useMemo(() => {
    if (!graph || !selectedNodeId) return null;
    return graph.nodes.find((n) => n.id === selectedNodeId) || null;
  }, [graph, selectedNodeId]);

  const selectedEdges = useMemo(() => {
    if (!graph || !selectedNodeId) return { incoming: [] as GraphEdge[], outgoing: [] as GraphEdge[] };
    return {
      incoming: graph.edges.filter((e) => e.to === selectedNodeId),
      outgoing: graph.edges.filter((e) => e.from === selectedNodeId),
    };
  }, [graph, selectedNodeId]);

  const viewNodes = useMemo(() => {
    if (!graph || !visibility) return [];
    return graph.nodes.filter((n) => visibility.viewNodeIds.has(n.id));
  }, [graph, visibility]);

  return (
    <div class="h-[100dvh] flex flex-col bg-stone-950 text-stone-100 select-none overflow-hidden">
      <header class="shrink-0 px-3 py-2 border-b border-white/10 bg-stone-950/95 flex flex-wrap items-center justify-between gap-2">
        <div class="flex items-center gap-2.5 min-w-0">
          <a
            href="./?view=arch"
            class="px-2.5 py-1 rounded-lg bg-stone-900 border border-white/10 text-xs text-stone-300 hover:text-white"
          >
            3D Arch
          </a>
          <a
            href="./"
            class="px-2.5 py-1 rounded-lg bg-stone-900 border border-white/10 text-xs text-stone-300 hover:text-white"
          >
            Schedule
          </a>
          <div class="min-w-0">
            <h1 class="text-sm font-bold text-stone-100 truncate">System Graph Explorer (Dev)</h1>
            <p class="text-[11px] text-stone-400 truncate">
              docs/system-graph.yaml ({graph?.nodes.length ?? 0} nodes / {graph?.edges.length ?? 0} edges)
            </p>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-1.5">
          <button
            type="button"
            onClick={() => setViewMode('overview')}
            class={`px-2.5 py-1 rounded-lg border text-xs font-medium cursor-pointer ${
              viewMode === 'overview'
                ? 'bg-rose-500/20 border-rose-400/50 text-rose-200'
                : 'bg-stone-900 border-white/10 text-stone-300'
            }`}
          >
            Overview (L1-L5)
          </button>
          <button
            type="button"
            onClick={() => setViewMode('dataflow')}
            class={`px-2.5 py-1 rounded-lg border text-xs font-medium cursor-pointer ${
              viewMode === 'dataflow'
                ? 'bg-emerald-500/20 border-emerald-400/50 text-emerald-200'
                : 'bg-stone-900 border-white/10 text-stone-300'
            }`}
          >
            Data & Concurrency
          </button>

          <span class="w-px h-5 bg-white/10 mx-0.5" />

          <button
            type="button"
            onClick={() => zoomCenter(1.25)}
            class="px-2 py-1 rounded-lg border border-white/10 bg-stone-900 text-xs text-stone-200 cursor-pointer"
          >
            +
          </button>
          <button
            type="button"
            onClick={() => zoomCenter(1 / 1.25)}
            class="px-2 py-1 rounded-lg border border-white/10 bg-stone-900 text-xs text-stone-200 cursor-pointer"
          >
            -
          </button>
          <button
            type="button"
            onClick={() => {
              const b = bindingsRef.current;
              if (b) fitToViewport(b.viewWidth, b.viewHeight);
            }}
            class="px-2 py-1 rounded-lg border border-white/10 bg-stone-900 text-xs text-stone-200 cursor-pointer"
          >
            Fit
          </button>
          <button
            type="button"
            onClick={() => setTransform({ x: 24, y: 24, scale: 1 })}
            class="px-2 py-1 rounded-lg border border-white/10 bg-stone-900 text-xs text-stone-200 cursor-pointer"
          >
            100% ({Math.round(transform.scale * 100)}%)
          </button>

          <span class="w-px h-5 bg-white/10 mx-0.5" />

          <button
            type="button"
            disabled={renderingD2}
            onClick={() => void handleRerenderD2()}
            class="px-2.5 py-1 rounded-lg border border-sky-400/40 bg-sky-500/15 text-xs text-sky-200 hover:bg-sky-500/25 disabled:opacity-50 cursor-pointer"
          >
            {renderingD2 ? 'Rendering...' : 'Re-render D2'}
          </button>
        </div>
      </header>

      {errorMsg && (
        <div class="px-3 py-2 bg-rose-950/80 border-b border-rose-500/40 text-xs text-rose-200">
          {errorMsg}
        </div>
      )}

      <div class="flex-1 min-h-0 flex">
        {/* Left Filter & Node Visibility Sidebar */}
        <aside class="w-72 shrink-0 border-r border-white/10 bg-stone-900/60 flex flex-col min-h-0">
          <div class="p-3 border-b border-white/10 space-y-2.5 overflow-y-auto max-h-[52%]">
            <input
              type="text"
              value={searchQuery}
              onInput={(e) => setSearchQuery((e.target as HTMLInputElement).value)}
              placeholder="Search id / label / file / target..."
              class="w-full px-2.5 py-1.5 rounded-lg bg-stone-950 border border-white/10 text-xs text-stone-100 placeholder:text-stone-500 focus:outline-none focus:border-sky-400"
            />

            <div>
              <div class="text-[11px] font-semibold text-stone-400 mb-1">Hidden Display</div>
              <div class="flex gap-1">
                {(['hide', 'dim'] as const).map((m) => (
                  <button
                    key={m}
                    type="button"
                    onClick={() => setHiddenMode(m)}
                    class={`flex-1 py-1 rounded border text-[11px] cursor-pointer ${
                      hiddenMode === m
                        ? 'bg-amber-500/20 border-amber-400/50 text-amber-200'
                        : 'bg-stone-950 border-white/10 text-stone-400'
                    }`}
                  >
                    {m === 'hide' ? 'Hide' : 'Dim'}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <div class="text-[11px] font-semibold text-stone-400 mb-1">Layers</div>
              <div class="flex flex-wrap gap-1">
                {ALL_LAYERS.map((l) => (
                  <button
                    key={l.id}
                    type="button"
                    onClick={() => toggleLayer(l.id)}
                    class={`px-2 py-0.5 rounded border text-[11px] cursor-pointer ${
                      layers.has(l.id)
                        ? 'bg-stone-800 border-white/25 text-stone-100'
                        : 'bg-stone-950 border-white/5 text-stone-500'
                    }`}
                  >
                    {l.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <div class="text-[11px] font-semibold text-stone-400 mb-1">Status Exclusions</div>
              <div class="space-y-1 text-xs">
                <label class="flex items-center gap-2 cursor-pointer text-stone-300">
                  <input type="checkbox" checked={hideSuspended} onChange={() => setHideSuspended((v) => !v)} />
                  <span>Hide suspended</span>
                </label>
                <label class="flex items-center gap-2 cursor-pointer text-stone-300">
                  <input type="checkbox" checked={hideLegacyMirrors} onChange={() => setHideLegacyMirrors((v) => !v)} />
                  <span>Hide legacy_mirror</span>
                </label>
                <label class="flex items-center gap-2 cursor-pointer text-stone-300">
                  <input type="checkbox" checked={hideManual} onChange={() => setHideManual((v) => !v)} />
                  <span>Hide manual CLI/Jobs</span>
                </label>
              </div>
            </div>

            {viewMode === 'overview' && (
              <div>
                <div class="text-[11px] font-semibold text-stone-400 mb-1">Edge Kinds</div>
                <div class="flex flex-wrap gap-1">
                  {ALL_EDGE_KINDS.map((k) => (
                    <button
                      key={k.id}
                      type="button"
                      onClick={() => toggleEdgeKind(k.id)}
                      class={`px-2 py-0.5 rounded border text-[11px] cursor-pointer ${
                        edgeKinds.has(k.id)
                          ? 'bg-stone-800 border-white/25 text-stone-100'
                          : 'bg-stone-950 border-white/5 text-stone-500'
                      }`}
                    >
                      {k.label}
                    </button>
                  ))}
                </div>
              </div>
            )}

            <div>
              <div class="text-[11px] font-semibold text-stone-400 mb-1">Data Concurrency Filter</div>
              <select
                value={concurrencyFilter}
                onChange={(e) => setConcurrencyFilter((e.target as HTMLSelectElement).value as ConcurrencyFilter)}
                class="w-full px-2 py-1 rounded bg-stone-950 border border-white/10 text-xs text-stone-200"
              >
                <option value="all">All edges</option>
                <option value="writes_only">Data writes only (write / rw)</option>
                <option value="none_put">PUT without lock (none_put)</option>
                <option value="generation_lock">Generation locked (generation_lock)</option>
              </select>
            </div>
          </div>

          <div class="px-3 py-2 border-b border-white/10 flex items-center justify-between text-[11px]">
            <span class="font-semibold text-stone-300">
              Nodes ({visibility?.visibleNodeIds.size ?? 0} / {viewNodes.length})
            </span>
            <div class="flex gap-2">
              <button
                type="button"
                onClick={() => setHiddenNodeIds(new Set())}
                class="text-sky-300 hover:underline cursor-pointer"
              >
                Show all
              </button>
              <button
                type="button"
                onClick={() => setHiddenNodeIds(new Set(viewNodes.map((n) => n.id)))}
                class="text-stone-400 hover:underline cursor-pointer"
              >
                Hide all
              </button>
            </div>
          </div>

          <div class="flex-1 overflow-y-auto p-2 space-y-1">
            {viewNodes.map((node) => {
              const hidden = hiddenNodeIds.has(node.id);
              const selected = selectedNodeId === node.id;
              const visible = visibility?.visibleNodeIds.has(node.id) ?? false;
              return (
                <div
                  key={node.id}
                  class={`flex items-center gap-1.5 px-2 py-1 rounded text-xs ${
                    selected ? 'bg-rose-500/20 border border-rose-400/40' : 'hover:bg-stone-800/70'
                  } ${!visible ? 'opacity-45' : ''}`}
                >
                  <input
                    type="checkbox"
                    checked={!hidden}
                    onChange={() => toggleNodeHidden(node.id)}
                    aria-label={`Toggle ${node.id}`}
                    class="cursor-pointer shrink-0"
                  />
                  <button
                    type="button"
                    onClick={() => setSelectedNodeId((prev) => (prev === node.id ? null : node.id))}
                    class="flex-1 text-left truncate cursor-pointer"
                    title={`${node.id}: ${node.label}`}
                  >
                    <span class="text-stone-200">{node.label}</span>
                  </button>
                  <span class="text-[10px] text-stone-500 shrink-0">{node.layer}</span>
                </div>
              );
            })}
          </div>
        </aside>

        {/* Interactive 2D SVG Pan/Zoom Viewport */}
        <main
          ref={viewportRef}
          onWheel={handleWheel}
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerUp}
          onClick={() => {
            if (!dragStateRef.current.moved) setSelectedNodeId(null);
          }}
          class="flex-1 min-w-0 relative overflow-hidden bg-[#1E1E2E] cursor-grab active:cursor-grabbing"
        >
          <div
            ref={svgHostRef}
            style={{
              transform: `translate(${transform.x}px, ${transform.y}px) scale(${transform.scale})`,
              transformOrigin: '0 0',
            }}
            class="absolute top-0 left-0 will-change-transform"
          />

          <div class="pointer-events-none absolute bottom-3 left-3 px-2.5 py-1 rounded bg-stone-950/80 border border-white/10 text-[11px] text-stone-300">
            Wheel: Zoom / Drag: Pan / Click node: Highlight 1-hop neighborhood
          </div>
        </main>

        {/* Right Node Inspector */}
        {selectedNode && (
          <aside class="w-80 shrink-0 border-l border-white/10 bg-stone-900/80 p-3 overflow-y-auto space-y-3 select-text">
            <div class="flex items-start justify-between gap-2">
              <div>
                <div class="text-[11px] font-mono text-stone-400">{selectedNode.id}</div>
                <h2 class="text-sm font-bold text-stone-100">{selectedNode.label}</h2>
              </div>
              <button
                type="button"
                onClick={() => setSelectedNodeId(null)}
                class="px-2 py-0.5 rounded bg-stone-800 text-xs text-stone-300 cursor-pointer"
              >
                Close
              </button>
            </div>

            <div class="flex flex-wrap gap-1.5 text-[11px]">
              <span class="px-2 py-0.5 rounded bg-stone-800 text-stone-200">{selectedNode.layer}</span>
              <span class="px-2 py-0.5 rounded bg-stone-800 text-stone-200">{selectedNode.kind}</span>
              <span class="px-2 py-0.5 rounded bg-stone-800 text-amber-200">{selectedNode.status}</span>
              {selectedNode.host_group && (
                <span class="px-2 py-0.5 rounded bg-stone-800 text-sky-200">host: {selectedNode.host_group}</span>
              )}
            </div>

            <div class="text-xs space-y-1">
              <div class="text-stone-400 font-semibold">Entry</div>
              <div class="font-mono text-[11px] text-stone-200 break-all">{selectedNode.entry}</div>
              {selectedNode.notes && <p class="text-stone-300 text-[11px] pt-1">{selectedNode.notes}</p>}
            </div>

            <button
              type="button"
              onClick={() => toggleNodeHidden(selectedNode.id)}
              class="w-full py-1.5 rounded border border-white/15 bg-stone-800 text-xs text-stone-200 hover:bg-stone-700 cursor-pointer"
            >
              {hiddenNodeIds.has(selectedNode.id) ? 'Show node on canvas' : 'Hide node on canvas'}
            </button>

            <div class="space-y-2">
              <div class="text-xs font-semibold text-stone-300">Outgoing ({selectedEdges.outgoing.length})</div>
              {selectedEdges.outgoing.map((e) => (
                <div key={e.id} class="p-2 rounded bg-stone-950/90 border border-white/10 text-[11px] space-y-1">
                  <div class="flex items-center justify-between gap-1">
                    <button
                      type="button"
                      onClick={() => setSelectedNodeId(e.to)}
                      class="font-mono text-sky-300 hover:underline cursor-pointer truncate"
                    >
                      -&gt; {e.to}
                    </button>
                    {e.concurrency && (
                      <span
                        class={`px-1.5 py-0.5 rounded text-[10px] ${
                          e.concurrency === 'none_put'
                            ? 'bg-rose-500/20 text-rose-200'
                            : e.concurrency === 'generation_lock'
                            ? 'bg-emerald-500/20 text-emerald-200'
                            : 'bg-stone-800 text-stone-300'
                        }`}
                      >
                        {e.concurrency}
                      </span>
                    )}
                  </div>
                  <div class="text-stone-400">
                    {e.kind}
                    {e.op ? ` (${e.op})` : ''} / {e.entry}
                  </div>
                  {e.targets && (
                    <div class="font-mono text-[10px] text-stone-300 break-all">{e.targets.join(', ')}</div>
                  )}
                </div>
              ))}
            </div>

            <div class="space-y-2">
              <div class="text-xs font-semibold text-stone-300">Incoming ({selectedEdges.incoming.length})</div>
              {selectedEdges.incoming.map((e) => (
                <div key={e.id} class="p-2 rounded bg-stone-950/90 border border-white/10 text-[11px] space-y-1">
                  <div class="flex items-center justify-between gap-1">
                    <button
                      type="button"
                      onClick={() => setSelectedNodeId(e.from)}
                      class="font-mono text-sky-300 hover:underline cursor-pointer truncate"
                    >
                      &lt;- {e.from}
                    </button>
                    {e.concurrency && (
                      <span
                        class={`px-1.5 py-0.5 rounded text-[10px] ${
                          e.concurrency === 'none_put'
                            ? 'bg-rose-500/20 text-rose-200'
                            : e.concurrency === 'generation_lock'
                            ? 'bg-emerald-500/20 text-emerald-200'
                            : 'bg-stone-800 text-stone-300'
                        }`}
                      >
                        {e.concurrency}
                      </span>
                    )}
                  </div>
                  <div class="text-stone-400">
                    {e.kind}
                    {e.op ? ` (${e.op})` : ''} / {e.entry}
                  </div>
                  {e.targets && (
                    <div class="font-mono text-[10px] text-stone-300 break-all">{e.targets.join(', ')}</div>
                  )}
                </div>
              ))}
            </div>
          </aside>
        )}
      </div>
    </div>
  );
}
