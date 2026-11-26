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

const ALL_LAYERS: { id: LayerId; label: string; accent: string; badge: string }[] = [
  { id: 'edge', label: 'L1 Edge', accent: 'border-rose-400/50', badge: 'text-rose-300' },
  { id: 'service', label: 'L2 Service', accent: 'border-amber-400/50', badge: 'text-amber-300' },
  { id: 'worker', label: 'L3 Worker', accent: 'border-sky-400/50', badge: 'text-sky-300' },
  { id: 'store', label: 'L4 Store', accent: 'border-emerald-400/50', badge: 'text-emerald-300' },
  { id: 'external', label: 'L5 External', accent: 'border-purple-400/50', badge: 'text-purple-300' },
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

interface BoardLine {
  edge: GraphEdge;
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  color: string;
  dashed: boolean;
  strokeWidth: number;
}

function edgeVisualProps(edge: GraphEdge): { color: string; dashed: boolean; width: number; shortLabel: string } {
  if (edge.status === 'suspended') {
    return { color: '#64748b', dashed: true, width: 1.5, shortLabel: `${edge.kind} (suspended)` };
  }
  if (edge.kind === 'data') {
    if (edge.concurrency === 'none_put') {
      return { color: '#fb7185', dashed: false, width: 3, shortLabel: `${edge.op?.toUpperCase()}: PUT !Lock` };
    }
    if (edge.concurrency === 'generation_lock') {
      return { color: '#34d399', dashed: false, width: 2.5, shortLabel: `${edge.op?.toUpperCase()}: IfGen` };
    }
    if (edge.concurrency === 'shared_pvc_single') {
      return { color: '#fbbf24', dashed: false, width: 2.5, shortLabel: `${edge.op?.toUpperCase()}: PVC` };
    }
    if (edge.concurrency === 'client_local') {
      return { color: '#cbd5e1', dashed: false, width: 2, shortLabel: 'LocalStorage' };
    }
    return { color: '#94a3b8', dashed: true, width: 2, shortLabel: 'READ' };
  }
  if (edge.kind === 'request') {
    return { color: '#38bdf8', dashed: false, width: 2.5, shortLabel: 'HTTP' };
  }
  return { color: '#c084fc', dashed: edge.status === 'manual', width: 2, shortLabel: 'invoke' };
}

export function DevGraphView() {
  const [graph, setGraph] = useState<SystemGraphData | null>(null);
  const [viewMode, setViewMode] = useState<GraphViewMode>('board');
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
  const [hoveredNodeId, setHoveredNodeId] = useState<string | null>(null);

  const [transform, setTransform] = useState({ x: 24, y: 24, scale: 0.45 });
  const viewportRef = useRef<HTMLDivElement>(null);
  const svgHostRef = useRef<HTMLDivElement>(null);
  const bindingsRef = useRef<ParsedSvgBindings | null>(null);
  const boardContainerRef = useRef<HTMLDivElement>(null);
  const cardRefs = useRef<Map<string, HTMLElement>>(new Map());
  const [boardLines, setBoardLines] = useState<{ width: number; height: number; items: BoardLine[] }>({
    width: 0,
    height: 0,
    items: [],
  });

  const dragStateRef = useRef<{ active: boolean; moved: boolean; startX: number; startY: number; origX: number; origY: number }>({
    active: false,
    moved: false,
    startX: 0,
    startY: 0,
    origX: 0,
    origY: 0,
  });

  const focusNodeId = selectedNodeId ?? hoveredNodeId;

  const loadGraphData = useCallback(async (mode: GraphViewMode) => {
    try {
      const yamlPromise = fetch('/__dev/system-graph.json', { cache: 'no-store' });
      if (mode === 'board') {
        const yamlRes = await yamlPromise;
        if (!yamlRes.ok) throw new Error(`system-graph.json HTTP ${yamlRes.status}`);
        setGraph((await yamlRes.json()) as SystemGraphData);
        setSvgMarkup('');
        setErrorMsg(null);
        return;
      }

      const svgPath = mode === 'overview' ? '/__dev/graph-overview.svg' : '/__dev/graph-dataflow.svg';
      const [yamlRes, svgRes] = await Promise.all([yamlPromise, fetch(svgPath, { cache: 'no-store' })]);
      if (!yamlRes.ok) throw new Error(`system-graph.json HTTP ${yamlRes.status}`);
      if (!svgRes.ok) throw new Error(`${svgPath} HTTP ${svgRes.status}`);
      setGraph((await yamlRes.json()) as SystemGraphData);
      setSvgMarkup(await svgRes.text());
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

  const focusNodeInSvg = useCallback((nodeId: string) => {
    const vp = viewportRef.current;
    const bindings = bindingsRef.current;
    const outerSvg = svgHostRef.current?.querySelector('svg');
    if (!vp || !bindings || !outerSvg) return;
    const els = bindings.nodeElements.get(nodeId);
    if (!els || els.length === 0) return;
    const elRect = els[0].getBoundingClientRect();
    const svgRect = outerSvg.getBoundingClientRect();
    if (svgRect.width <= 0) return;

    const currentScale = svgRect.width / bindings.viewWidth;
    const cxInSvg = (elRect.left + elRect.width / 2 - svgRect.left) / currentScale;
    const cyInSvg = (elRect.top + elRect.height / 2 - svgRect.top) / currentScale;

    const targetScale = Math.max(currentScale, 0.85);
    setTransform({
      scale: targetScale,
      x: vp.clientWidth / 2 - cxInSvg * targetScale,
      y: vp.clientHeight / 2 - cyInSvg * targetScale,
    });
  }, []);

  useEffect(() => {
    const host = svgHostRef.current;
    if (viewMode === 'board' || !host || !svgMarkup || !graph) return;

    host.innerHTML = svgMarkup;
    const outerSvg = host.querySelector('svg');
    if (!outerSvg) return;

    const vb = outerSvg.viewBox.baseVal;
    const viewWidth = vb && vb.width > 0 ? vb.width : Number(outerSvg.getAttribute('width')) || 2400;
    const viewHeight = vb && vb.height > 0 ? vb.height : Number(outerSvg.getAttribute('height')) || 1400;
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
        const onDblClick = (ev: MouseEvent) => {
          ev.stopPropagation();
          setSelectedNodeId(target.nodeId);
          focusNodeInSvg(target.nodeId);
        };
        g.addEventListener('click', onClick);
        g.addEventListener('dblclick', onDblClick);
        cleanups.push(() => {
          g.removeEventListener('click', onClick);
          g.removeEventListener('dblclick', onDblClick);
        });
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
  }, [svgMarkup, graph, viewMode, fitToViewport, focusNodeInSvg]);

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
      focusNodeId,
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
    focusNodeId,
    viewMode,
  ]);

  useEffect(() => {
    const bindings = bindingsRef.current;
    if (viewMode === 'board' || !bindings || !visibility) return;

    for (const [nodeId, els] of bindings.nodeElements.entries()) {
      const isVisible = visibility.visibleNodeIds.has(nodeId);
      const isDimmed = isVisible && visibility.dimmedNodeIds.has(nodeId);
      const isSelected = selectedNodeId === nodeId;
      for (const el of els) {
        if (!isVisible) {
          el.style.display = hiddenMode === 'hide' ? 'none' : '';
          el.style.opacity = hiddenMode === 'dim' ? '0.08' : '1';
          el.style.pointerEvents = hiddenMode === 'dim' ? 'none' : '';
          el.style.filter = '';
        } else {
          el.style.display = '';
          el.style.opacity = isDimmed ? '0.16' : '1';
          el.style.pointerEvents = '';
          el.style.filter = isSelected ? 'drop-shadow(0 0 10px rgba(56, 189, 248, 0.9))' : '';
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
        el.style.opacity = isDimmed ? '0.10' : '1';
      }
    }
  }, [visibility, hiddenMode, svgMarkup, selectedNodeId, viewMode]);

  const recomputeBoardLines = useCallback(() => {
    const container = boardContainerRef.current;
    if (viewMode !== 'board' || !container || !graph || !visibility) return;
    const cRect = container.getBoundingClientRect();
    const scrollLeft = container.scrollLeft;
    const scrollTop = container.scrollTop;

    const edgesToDraw = graph.edges.filter((e) => {
      if (!visibility.visibleEdgeIds.has(e.id)) return false;
      if (focusNodeId) {
        return e.from === focusNodeId || e.to === focusNodeId;
      }
      // Without selection, keep data/request edges subtle or only draw filtered subset when filtered.
      return true;
    });

    const items: BoardLine[] = [];
    for (const edge of edgesToDraw) {
      const srcEl = cardRefs.current.get(edge.from);
      const dstEl = cardRefs.current.get(edge.to);
      if (!srcEl || !dstEl) continue;
      const sr = srcEl.getBoundingClientRect();
      const dr = dstEl.getBoundingClientRect();
      const props = edgeVisualProps(edge);

      const srMidX = sr.left - cRect.left + scrollLeft + sr.width / 2;
      const drMidX = dr.left - cRect.left + scrollLeft + dr.width / 2;
      const x1 = drMidX >= srMidX ? sr.right - cRect.left + scrollLeft : sr.left - cRect.left + scrollLeft;
      const x2 = drMidX >= srMidX ? dr.left - cRect.left + scrollLeft : sr.right - cRect.left + scrollLeft;
      const y1 = sr.top - cRect.top + scrollTop + sr.height / 2;
      const y2 = dr.top - cRect.top + scrollTop + dr.height / 2;

      items.push({
        edge,
        x1,
        y1,
        x2,
        y2,
        color: props.color,
        dashed: props.dashed,
        strokeWidth: props.width,
      });
    }

    setBoardLines({
      width: container.scrollWidth,
      height: container.scrollHeight,
      items,
    });
  }, [viewMode, graph, visibility, focusNodeId]);

  useEffect(() => {
    if (viewMode !== 'board') return;
    const raf = requestAnimationFrame(recomputeBoardLines);
    window.addEventListener('resize', recomputeBoardLines);
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', recomputeBoardLines);
    };
  }, [viewMode, recomputeBoardLines]);

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
    if (viewMode === 'board') return;
    e.preventDefault();
    const vp = viewportRef.current;
    if (!vp) return;
    const rect = vp.getBoundingClientRect();
    const cx = e.clientX - rect.left;
    const cy = e.clientY - rect.top;
    const factor = e.deltaY < 0 ? 1.15 : 1 / 1.15;

    setTransform((prev) => {
      const nextScale = Math.max(0.12, Math.min(4.5, prev.scale * factor));
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
      const nextScale = Math.max(0.12, Math.min(4.5, prev.scale * factor));
      const ratio = nextScale / prev.scale;
      return {
        scale: nextScale,
        x: cx - (cx - prev.x) * ratio,
        y: cy - (cy - prev.y) * ratio,
      };
    });
  };

  const handlePointerDown = (e: PointerEvent) => {
    if (viewMode === 'board' || e.button !== 0) return;
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

  const edgeStatsByNode = useMemo(() => {
    const map = new Map<string, { inCount: number; outCount: number; hasNonePut: boolean; hasIfGen: boolean }>();
    if (!graph) return map;
    for (const n of graph.nodes) {
      map.set(n.id, { inCount: 0, outCount: 0, hasNonePut: false, hasIfGen: false });
    }
    for (const e of graph.edges) {
      const f = map.get(e.from);
      const t = map.get(e.to);
      if (f) {
        f.outCount += 1;
        if (e.concurrency === 'none_put') f.hasNonePut = true;
        if (e.concurrency === 'generation_lock') f.hasIfGen = true;
      }
      if (t) {
        t.inCount += 1;
        if (e.concurrency === 'none_put') t.hasNonePut = true;
        if (e.concurrency === 'generation_lock') t.hasIfGen = true;
      }
    }
    return map;
  }, [graph]);

  return (
    <div class="h-[100dvh] flex flex-col bg-slate-950 text-slate-100 select-none overflow-hidden">
      <header class="shrink-0 px-3 py-2 border-b border-slate-800 bg-slate-950 flex flex-wrap items-center justify-between gap-2">
        <div class="flex items-center gap-2 min-w-0">
          <a
            href="./?view=arch"
            class="px-2.5 py-1 rounded-lg bg-slate-900 border border-slate-700 text-xs text-slate-300 hover:text-white"
          >
            3D Arch
          </a>
          <a
            href="./"
            class="px-2.5 py-1 rounded-lg bg-slate-900 border border-slate-700 text-xs text-slate-300 hover:text-white"
          >
            Schedule
          </a>
          <div class="min-w-0 pl-1">
            <h1 class="text-sm font-bold text-slate-100 truncate">System Graph Explorer (Dev)</h1>
            <p class="text-[11px] text-slate-400 truncate">
              docs/system-graph.yaml ({graph?.nodes.length ?? 0} nodes / {graph?.edges.length ?? 0} edges)
            </p>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-1.5">
          <button
            type="button"
            onClick={() => setViewMode('board')}
            class={`px-2.5 py-1 rounded-lg border text-xs font-medium cursor-pointer ${
              viewMode === 'board'
                ? 'bg-sky-500/20 border-sky-400/60 text-sky-200'
                : 'bg-slate-900 border-slate-700 text-slate-300 hover:border-slate-500'
            }`}
          >
            Board (Layers)
          </button>
          <button
            type="button"
            onClick={() => setViewMode('dataflow')}
            class={`px-2.5 py-1 rounded-lg border text-xs font-medium cursor-pointer ${
              viewMode === 'dataflow'
                ? 'bg-emerald-500/20 border-emerald-400/60 text-emerald-200'
                : 'bg-slate-900 border-slate-700 text-slate-300 hover:border-slate-500'
            }`}
          >
            Dataflow (D2)
          </button>
          <button
            type="button"
            onClick={() => setViewMode('overview')}
            class={`px-2.5 py-1 rounded-lg border text-xs font-medium cursor-pointer ${
              viewMode === 'overview'
                ? 'bg-rose-500/20 border-rose-400/60 text-rose-200'
                : 'bg-slate-900 border-slate-700 text-slate-300 hover:border-slate-500'
            }`}
          >
            Overview (D2)
          </button>

          {viewMode !== 'board' && (
            <>
              <span class="w-px h-5 bg-slate-800 mx-0.5" />
              <button
                type="button"
                onClick={() => zoomCenter(1.25)}
                class="px-2 py-1 rounded-lg border border-slate-700 bg-slate-900 text-xs text-slate-200 cursor-pointer"
              >
                +
              </button>
              <button
                type="button"
                onClick={() => zoomCenter(1 / 1.25)}
                class="px-2 py-1 rounded-lg border border-slate-700 bg-slate-900 text-xs text-slate-200 cursor-pointer"
              >
                -
              </button>
              <button
                type="button"
                onClick={() => {
                  const b = bindingsRef.current;
                  if (b) fitToViewport(b.viewWidth, b.viewHeight);
                }}
                class="px-2 py-1 rounded-lg border border-slate-700 bg-slate-900 text-xs text-slate-200 cursor-pointer"
              >
                Fit
              </button>
              <button
                type="button"
                onClick={() => setTransform({ x: 24, y: 24, scale: 1 })}
                class="px-2 py-1 rounded-lg border border-slate-700 bg-slate-900 text-xs text-slate-200 cursor-pointer"
              >
                {Math.round(transform.scale * 100)}%
              </button>
            </>
          )}

          <span class="w-px h-5 bg-slate-800 mx-0.5" />

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
        <aside class="w-72 shrink-0 border-r border-slate-800 bg-slate-900/70 flex flex-col min-h-0">
          <div class="p-3 border-b border-slate-800 space-y-2.5 overflow-y-auto max-h-[54%]">
            <input
              type="text"
              value={searchQuery}
              onInput={(e) => setSearchQuery((e.target as HTMLInputElement).value)}
              placeholder="Search id / label / file / target..."
              class="w-full px-2.5 py-1.5 rounded-lg bg-slate-950 border border-slate-700 text-xs text-slate-100 placeholder:text-slate-500 focus:outline-none focus:border-sky-400"
            />

            <div>
              <div class="text-[11px] font-semibold text-slate-400 mb-1">Hidden Display</div>
              <div class="flex gap-1">
                {(['hide', 'dim'] as const).map((m) => (
                  <button
                    key={m}
                    type="button"
                    onClick={() => setHiddenMode(m)}
                    class={`flex-1 py-1 rounded border text-[11px] cursor-pointer ${
                      hiddenMode === m
                        ? 'bg-amber-500/20 border-amber-400/50 text-amber-200'
                        : 'bg-slate-950 border-slate-700 text-slate-400'
                    }`}
                  >
                    {m === 'hide' ? 'Hide' : 'Dim'}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <div class="text-[11px] font-semibold text-slate-400 mb-1">Layers</div>
              <div class="flex flex-wrap gap-1">
                {ALL_LAYERS.map((l) => (
                  <button
                    key={l.id}
                    type="button"
                    onClick={() => toggleLayer(l.id)}
                    class={`px-2 py-0.5 rounded border text-[11px] cursor-pointer ${
                      layers.has(l.id)
                        ? 'bg-slate-800 border-slate-500 text-slate-100'
                        : 'bg-slate-950 border-slate-800 text-slate-500'
                    }`}
                  >
                    {l.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <div class="text-[11px] font-semibold text-slate-400 mb-1">Status Exclusions</div>
              <div class="space-y-1 text-xs">
                <label class="flex items-center gap-2 cursor-pointer text-slate-300">
                  <input type="checkbox" checked={hideSuspended} onChange={() => setHideSuspended((v) => !v)} />
                  <span>Hide suspended</span>
                </label>
                <label class="flex items-center gap-2 cursor-pointer text-slate-300">
                  <input type="checkbox" checked={hideLegacyMirrors} onChange={() => setHideLegacyMirrors((v) => !v)} />
                  <span>Hide legacy_mirror</span>
                </label>
                <label class="flex items-center gap-2 cursor-pointer text-slate-300">
                  <input type="checkbox" checked={hideManual} onChange={() => setHideManual((v) => !v)} />
                  <span>Hide manual CLI/Jobs</span>
                </label>
              </div>
            </div>

            {viewMode !== 'dataflow' && (
              <div>
                <div class="text-[11px] font-semibold text-slate-400 mb-1">Edge Kinds</div>
                <div class="flex flex-wrap gap-1">
                  {ALL_EDGE_KINDS.map((k) => (
                    <button
                      key={k.id}
                      type="button"
                      onClick={() => toggleEdgeKind(k.id)}
                      class={`px-2 py-0.5 rounded border text-[11px] cursor-pointer ${
                        edgeKinds.has(k.id)
                          ? 'bg-slate-800 border-slate-500 text-slate-100'
                          : 'bg-slate-950 border-slate-800 text-slate-500'
                      }`}
                    >
                      {k.label}
                    </button>
                  ))}
                </div>
              </div>
            )}

            <div>
              <div class="text-[11px] font-semibold text-slate-400 mb-1">Data Concurrency Filter</div>
              <select
                value={concurrencyFilter}
                onChange={(e) => setConcurrencyFilter((e.target as HTMLSelectElement).value as ConcurrencyFilter)}
                class="w-full px-2 py-1 rounded bg-slate-950 border border-slate-700 text-xs text-slate-200"
              >
                <option value="all">All edges</option>
                <option value="writes_only">Data writes only (write / rw)</option>
                <option value="none_put">PUT without lock (none_put)</option>
                <option value="generation_lock">Generation locked (generation_lock)</option>
              </select>
            </div>
          </div>

          <div class="px-3 py-2 border-b border-slate-800 flex items-center justify-between text-[11px]">
            <span class="font-semibold text-slate-300">
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
                class="text-slate-400 hover:underline cursor-pointer"
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
                    selected ? 'bg-sky-500/20 border border-sky-400/50' : 'hover:bg-slate-800/70'
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
                    onClick={() => {
                      setSelectedNodeId((prev) => (prev === node.id ? null : node.id));
                      if (viewMode !== 'board') focusNodeInSvg(node.id);
                    }}
                    class="flex-1 text-left truncate cursor-pointer"
                    title={`${node.id}: ${node.label}`}
                  >
                    <span class="text-slate-100">{node.label}</span>
                  </button>
                  <span class="text-[10px] text-slate-500 shrink-0">{node.layer}</span>
                </div>
              );
            })}
          </div>
        </aside>

        {/* Main Viewport: either 5-Column High-Legibility Board OR D2 Pan/Zoom Canvas */}
        {viewMode === 'board' ? (
          <main
            ref={boardContainerRef}
            onScroll={recomputeBoardLines}
            onClick={() => setSelectedNodeId(null)}
            class="flex-1 min-w-0 relative overflow-auto bg-[#060913] p-4"
          >
            <svg
              width={boardLines.width}
              height={boardLines.height}
              class={`pointer-events-none absolute top-0 left-0 ${focusNodeId ? 'z-30' : 'z-10'}`}
            >
              <defs>
                <marker id="arrow-head" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                  <path d="M 0 1 L 10 5 L 0 9 z" fill="currentColor" />
                </marker>
              </defs>
              {boardLines.items.map((item) => {
                const dx = Math.max(40, Math.abs(item.x2 - item.x1) * 0.45);
                const c1x = item.x2 >= item.x1 ? item.x1 + dx : item.x1 - dx;
                const c2x = item.x2 >= item.x1 ? item.x2 - dx : item.x2 + dx;
                const d = `M ${item.x1} ${item.y1} C ${c1x} ${item.y1}, ${c2x} ${item.y2}, ${item.x2} ${item.y2}`;
                const focused = focusNodeId !== null;
                const props = edgeVisualProps(item.edge);
                const midX = (item.x1 + item.x2) / 2;
                const midY = (item.y1 + item.y2) / 2;
                return (
                  <g key={item.edge.id} style={{ color: item.color, opacity: focused ? 0.95 : 0.24 }}>
                    <path
                      d={d}
                      fill="none"
                      stroke={item.color}
                      stroke-width={focused ? item.strokeWidth : 1.5}
                      stroke-dasharray={item.dashed ? '5 4' : undefined}
                      marker-end="url(#arrow-head)"
                    />
                    {focused && (
                      <g transform={`translate(${midX}, ${midY})`}>
                        <rect x="-48" y="-10" width="96" height="18" rx="4" fill="#020617" stroke={item.color} stroke-width="1" />
                        <text x="0" y="3" text-anchor="middle" fill="#f8fafc" font-size="10" font-weight="600">
                          {props.shortLabel}
                        </text>
                      </g>
                    )}
                  </g>
                );
              })}
            </svg>

            <div class="relative z-20 grid grid-cols-5 gap-6 min-w-[1160px] items-start">
              {ALL_LAYERS.map((layerMeta) => {
                const layerNodes = (graph?.nodes ?? []).filter((n) => n.layer === layerMeta.id);
                return (
                  <section key={layerMeta.id} class="rounded-xl border border-slate-800 bg-slate-900/50 p-2.5 space-y-2">
                    <div class="flex items-center justify-between px-1 pb-1 border-b border-slate-800">
                      <h2 class={`text-xs font-bold tracking-wide ${layerMeta.badge}`}>{layerMeta.label}</h2>
                      <span class="text-[11px] text-slate-500">
                        {layerNodes.filter((n) => visibility?.visibleNodeIds.has(n.id)).length}/{layerNodes.length}
                      </span>
                    </div>

                    <div class="space-y-2">
                      {layerNodes.map((node) => {
                        const isVisible = visibility?.visibleNodeIds.has(node.id) ?? true;
                        if (!isVisible && hiddenMode === 'hide') return null;

                        const isDimmed = (!isVisible && hiddenMode === 'dim') || Boolean(focusNodeId && visibility?.dimmedNodeIds.has(node.id));
                        const isSelected = selectedNodeId === node.id;
                        const stats = edgeStatsByNode.get(node.id);

                        return (
                          <div
                            key={node.id}
                            ref={(el) => {
                              if (el) cardRefs.current.set(node.id, el);
                              else cardRefs.current.delete(node.id);
                            }}
                            onMouseEnter={() => setHoveredNodeId(node.id)}
                            onMouseLeave={() => setHoveredNodeId((prev) => (prev === node.id ? null : prev))}
                            onClick={(e) => {
                              e.stopPropagation();
                              setSelectedNodeId((prev) => (prev === node.id ? null : node.id));
                            }}
                            class={`rounded-lg border p-2.5 bg-slate-900 cursor-pointer transition-all ${
                              isSelected
                                ? 'border-sky-400 ring-2 ring-sky-400/40 bg-slate-800'
                                : `${layerMeta.accent} hover:border-slate-400`
                            } ${isDimmed ? 'opacity-25' : 'opacity-100'}`}
                          >
                            <div class="flex items-start justify-between gap-1.5">
                              <div class="text-xs font-semibold text-slate-100 leading-snug">{node.label}</div>
                              {node.status !== 'active' && (
                                <span class="px-1.5 py-0.5 rounded bg-slate-800 text-[10px] text-slate-300 shrink-0">
                                  {node.status}
                                </span>
                              )}
                            </div>

                            <div class="mt-1 text-[11px] font-mono text-slate-400 truncate">{node.entry}</div>

                            <div class="mt-2 flex flex-wrap items-center gap-1 text-[10px]">
                              {node.host_group && (
                                <span class="px-1.5 py-0.5 rounded bg-amber-500/15 text-amber-200 border border-amber-500/30">
                                  {node.host_group}
                                </span>
                              )}
                              {stats?.hasNonePut && (
                                <span class="px-1.5 py-0.5 rounded bg-rose-500/20 text-rose-200 border border-rose-500/30">
                                  PUT !Lock
                                </span>
                              )}
                              {stats?.hasIfGen && (
                                <span class="px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-200 border border-emerald-500/30">
                                  IfGen
                                </span>
                              )}
                              <span class="ml-auto text-slate-500">
                                {stats?.inCount ?? 0}in / {stats?.outCount ?? 0}out
                              </span>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </section>
                );
              })}
            </div>
          </main>
        ) : (
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
            class="flex-1 min-w-0 relative overflow-hidden bg-[#000410] cursor-grab active:cursor-grabbing"
          >
            <div
              ref={svgHostRef}
              style={{
                transform: `translate(${transform.x}px, ${transform.y}px) scale(${transform.scale})`,
                transformOrigin: '0 0',
              }}
              class="absolute top-0 left-0 will-change-transform"
            />

            <div class="pointer-events-none absolute bottom-3 left-3 px-2.5 py-1 rounded bg-slate-950/85 border border-slate-700 text-[11px] text-slate-300">
              Wheel: Zoom / Drag: Pan / Click: 1-hop highlight / Double-click: Center zoom
            </div>
          </main>
        )}

        {/* Right Node Inspector */}
        {selectedNode && (
          <aside class="w-80 shrink-0 border-l border-slate-800 bg-slate-900/90 p-3 overflow-y-auto space-y-3 select-text">
            <div class="flex items-start justify-between gap-2">
              <div>
                <div class="text-[11px] font-mono text-slate-400">{selectedNode.id}</div>
                <h2 class="text-sm font-bold text-slate-100">{selectedNode.label}</h2>
              </div>
              <button
                type="button"
                onClick={() => setSelectedNodeId(null)}
                class="px-2 py-0.5 rounded bg-slate-800 text-xs text-slate-300 cursor-pointer"
              >
                Close
              </button>
            </div>

            <div class="flex flex-wrap gap-1.5 text-[11px]">
              <span class="px-2 py-0.5 rounded bg-slate-800 text-slate-200">{selectedNode.layer}</span>
              <span class="px-2 py-0.5 rounded bg-slate-800 text-slate-200">{selectedNode.kind}</span>
              <span class="px-2 py-0.5 rounded bg-slate-800 text-amber-200">{selectedNode.status}</span>
              {selectedNode.host_group && (
                <span class="px-2 py-0.5 rounded bg-slate-800 text-sky-200">host: {selectedNode.host_group}</span>
              )}
            </div>

            <div class="text-xs space-y-1">
              <div class="text-slate-400 font-semibold">Entry</div>
              <div class="font-mono text-[11px] text-slate-200 break-all">{selectedNode.entry}</div>
              {selectedNode.notes && <p class="text-slate-300 text-[11px] pt-1">{selectedNode.notes}</p>}
            </div>

            <div class="flex gap-2">
              {viewMode !== 'board' && (
                <button
                  type="button"
                  onClick={() => focusNodeInSvg(selectedNode.id)}
                  class="flex-1 py-1.5 rounded border border-sky-400/40 bg-sky-500/15 text-xs text-sky-200 hover:bg-sky-500/25 cursor-pointer"
                >
                  Focus Zoom
                </button>
              )}
              <button
                type="button"
                onClick={() => toggleNodeHidden(selectedNode.id)}
                class="flex-1 py-1.5 rounded border border-slate-600 bg-slate-800 text-xs text-slate-200 hover:bg-slate-700 cursor-pointer"
              >
                {hiddenNodeIds.has(selectedNode.id) ? 'Show node' : 'Hide node'}
              </button>
            </div>

            <div class="space-y-2">
              <div class="text-xs font-semibold text-slate-300">Outgoing ({selectedEdges.outgoing.length})</div>
              {selectedEdges.outgoing.map((e) => (
                <div key={e.id} class="p-2 rounded bg-slate-950/90 border border-slate-800 text-[11px] space-y-1">
                  <div class="flex items-center justify-between gap-1">
                    <button
                      type="button"
                      onClick={() => {
                        setSelectedNodeId(e.to);
                        if (viewMode !== 'board') focusNodeInSvg(e.to);
                      }}
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
                            : 'bg-slate-800 text-slate-300'
                        }`}
                      >
                        {e.concurrency}
                      </span>
                    )}
                  </div>
                  <div class="text-slate-400">
                    {e.kind}
                    {e.op ? ` (${e.op})` : ''} / {e.entry}
                  </div>
                  {e.targets && (
                    <div class="font-mono text-[10px] text-slate-300 break-all">{e.targets.join(', ')}</div>
                  )}
                </div>
              ))}
            </div>

            <div class="space-y-2">
              <div class="text-xs font-semibold text-slate-300">Incoming ({selectedEdges.incoming.length})</div>
              {selectedEdges.incoming.map((e) => (
                <div key={e.id} class="p-2 rounded bg-slate-950/90 border border-slate-800 text-[11px] space-y-1">
                  <div class="flex items-center justify-between gap-1">
                    <button
                      type="button"
                      onClick={() => {
                        setSelectedNodeId(e.from);
                        if (viewMode !== 'board') focusNodeInSvg(e.from);
                      }}
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
                            : 'bg-slate-800 text-slate-300'
                        }`}
                      >
                        {e.concurrency}
                      </span>
                    )}
                  </div>
                  <div class="text-slate-400">
                    {e.kind}
                    {e.op ? ` (${e.op})` : ''} / {e.entry}
                  </div>
                  {e.targets && (
                    <div class="font-mono text-[10px] text-slate-300 break-all">{e.targets.join(', ')}</div>
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
