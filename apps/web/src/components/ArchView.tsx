import { useEffect, useMemo, useRef, useState } from 'preact/hooks';
import * as THREE from 'three';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
import { CSS2DObject, CSS2DRenderer } from 'three/examples/jsm/renderers/CSS2DRenderer.js';
import manifest from '../../../../docs/architecture.manifest.json';
import { Icon } from './Icon';
import {
  ArrowRight,
  Box,
  Database,
  GitBranch,
  Layers,
  RotateCcw,
  Server,
  Shield,
} from '../utils/icons';

type LayerId = 'edge' | 'app' | 'pipeline' | 'storage';
type ReviewMode = 'pipelines' | 'boundaries' | 'overview';
type CameraPreset = 'iso' | 'top' | 'front';

interface ArchComponent {
  id: string;
  label: string;
  layer: LayerId;
  path: string;
  dependsOn: string[];
}

interface ArchBoundary {
  layer: string;
  level: 'shared' | 'host_routed' | 'path_isolated' | 'prefix_isolated';
  details: string;
  path: string;
}

interface ArchTenant {
  id: string;
  hosts: string[];
  dataUrl: string;
  fallbackDataUrl: string;
  storagePrefix: string;
}

interface PipelineStep {
  id: string;
  label: string;
  component: string;
  entryPath: string;
  summary: string;
  guards: string[];
  stores?: string[];
}

interface ArchPipeline {
  id: string;
  name: string;
  mode: 'pull' | 'manual' | 'cron' | 'reconcile';
  description: string;
  steps: PipelineStep[];
}

interface ArchManifest {
  components: ArchComponent[];
  boundaries: ArchBoundary[];
  tenants: ArchTenant[];
  pipelines: ArchPipeline[];
}

const ARCH = manifest as ArchManifest;

const LAYER_CONFIG: Record<
  LayerId,
  { title: string; y: number; staggerX: number; hex: number; accentClass: string; badgeClass: string }
> = {
  edge: {
    title: 'Edge & Client',
    y: 5.4,
    staggerX: -1.4,
    hex: 0xfb7185,
    accentClass: 'border-rose-400/50',
    badgeClass: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
  },
  app: {
    title: 'App & API Pod',
    y: 1.8,
    staggerX: 1.4,
    hex: 0xf59e0b,
    accentClass: 'border-amber-400/50',
    badgeClass: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
  },
  pipeline: {
    title: 'Ingestion Pipelines',
    y: -1.8,
    staggerX: -1.2,
    hex: 0x38bdf8,
    accentClass: 'border-sky-400/50',
    badgeClass: 'bg-sky-500/15 text-sky-300 border-sky-500/30',
  },
  storage: {
    title: 'State & Storage',
    y: -5.4,
    staggerX: 3.2,
    hex: 0x34d399,
    accentClass: 'border-emerald-400/50',
    badgeClass: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
  },
};

const MODE_META: Record<
  ArchPipeline['mode'],
  { label: string; hex: number; pillClass: string }
> = {
  pull: {
    label: 'PULL / STAGED',
    hex: 0xfb7185,
    pillClass: 'bg-rose-500/15 text-rose-300 border-rose-500/30',
  },
  manual: {
    label: 'MANUAL / AI',
    hex: 0xf59e0b,
    pillClass: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
  },
  cron: {
    label: 'CRON / CLI',
    hex: 0x38bdf8,
    pillClass: 'bg-sky-500/15 text-sky-300 border-sky-500/30',
  },
  reconcile: {
    label: 'RECONCILE',
    hex: 0x34d399,
    pillClass: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30',
  },
};

const BOUNDARY_META: Record<
  ArchBoundary['level'],
  { label: string; pillClass: string }
> = {
  shared: {
    label: 'SHARED',
    pillClass: 'bg-amber-500/15 text-amber-300 border-amber-500/35',
  },
  host_routed: {
    label: 'HOST ROUTED',
    pillClass: 'bg-indigo-500/15 text-indigo-300 border-indigo-500/35',
  },
  path_isolated: {
    label: 'PATH ISOLATED',
    pillClass: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/35',
  },
  prefix_isolated: {
    label: 'PREFIX ISOLATED',
    pillClass: 'bg-emerald-500/15 text-emerald-300 border-emerald-500/35',
  },
};

const CAMERA_PRESETS: Record<CameraPreset, { pos: [number, number, number]; target: [number, number, number] }> = {
  iso: { pos: [14.5, 13.5, 24.5], target: [0, -0.3, 0] },
  top: { pos: [0.1, 28.0, 6.0], target: [0, 0, 0] },
  front: { pos: [0, 1.5, 26.0], target: [0, -0.2, 0] },
};

function computeNodePositions(components: ArchComponent[]): Map<string, THREE.Vector3> {
  const byLayer = new Map<LayerId, ArchComponent[]>([
    ['edge', []],
    ['app', []],
    ['pipeline', []],
    ['storage', []],
  ]);

  for (const comp of components) {
    byLayer.get(comp.layer)?.push(comp);
  }

  const positions = new Map<string, THREE.Vector3>();
  for (const [layer, items] of byLayer.entries()) {
    const { y, staggerX } = LAYER_CONFIG[layer];
    const cols = items.length <= 3 ? items.length : Math.ceil(items.length / 2);
    const rows = Math.ceil(items.length / Math.max(cols, 1));
    items.forEach((item, idx) => {
      const col = idx % Math.max(cols, 1);
      const row = Math.floor(idx / Math.max(cols, 1));
      const x = (col - (cols - 1) / 2) * 7.2 + staggerX;
      const z = (row - (rows - 1) / 2) * 5.6;
      positions.set(item.id, new THREE.Vector3(x, y, z));
    });
  }
  return positions;
}

function buildEdgeCurve(from: THREE.Vector3, to: THREE.Vector3, arcBoost = 0): THREE.CatmullRomCurve3 {
  const mid = new THREE.Vector3().addVectors(from, to).multiplyScalar(0.5);
  const dx = to.x - from.x;
  const dz = to.z - from.z;
  const sideOffset = Math.abs(from.y - to.y) < 0.2 ? 1.1 : 0.7;
  mid.x += (dz >= 0 ? 1 : -1) * (0.45 + arcBoost * 0.25);
  mid.z += (dx >= 0 ? -1 : 1) * sideOffset;
  mid.y += (from.y === to.y ? 0.9 : 0) + arcBoost * 0.15;
  return new THREE.CatmullRomCurve3([from.clone(), mid, to.clone()]);
}

export function ArchView() {
  const [reviewMode, setReviewMode] = useState<ReviewMode>('pipelines');
  const [selectedPipelineId, setSelectedPipelineId] = useState<string>(ARCH.pipelines[0]?.id ?? '');
  const [selectedComponentId, setSelectedComponentId] = useState<string | null>(
    ARCH.pipelines[0]?.steps[0]?.component ?? ARCH.components[0]?.id ?? null,
  );
  const [selectedStepId, setSelectedStepId] = useState<string | null>(
    ARCH.pipelines[0]?.steps[0]?.id ?? null,
  );
  const [autoRotate, setAutoRotate] = useState(false);
  const [webglError, setWebglError] = useState<string | null>(null);

  const stageRef = useRef<HTMLDivElement | null>(null);
  const controlsRef = useRef<OrbitControls | null>(null);
  const cameraRef = useRef<THREE.PerspectiveCamera | null>(null);
  const cardElsRef = useRef<Map<string, HTMLButtonElement>>(new Map());
  const depEdgesRef = useRef<Array<{ from: string; to: string; line: THREE.Line }>>([]);
  const flowGroupRef = useRef<THREE.Group | null>(null);
  const flowSegmentsRef = useRef<
    Array<{ curve: THREE.CatmullRomCurve3; orb: THREE.Mesh; offset: number }>
  >([]);
  const autoRotateRef = useRef(autoRotate);
  autoRotateRef.current = autoRotate;

  const positions = useMemo(() => computeNodePositions(ARCH.components), []);

  const activePipeline = useMemo(
    () => ARCH.pipelines.find((p) => p.id === selectedPipelineId) ?? null,
    [selectedPipelineId],
  );

  const selectedComponent = useMemo(
    () => ARCH.components.find((c) => c.id === selectedComponentId) ?? null,
    [selectedComponentId],
  );

  const relatedSteps = useMemo(() => {
    if (!selectedComponentId) return [];
    const hits: Array<{ pipeline: ArchPipeline; step: ArchPipeline['steps'][number]; index: number }> = [];
    for (const pipeline of ARCH.pipelines) {
      pipeline.steps.forEach((step, idx) => {
        if (step.component === selectedComponentId) {
          hits.push({ pipeline, step, index: idx + 1 });
        }
      });
    }
    return hits;
  }, [selectedComponentId]);

  const applyCameraPreset = (preset: CameraPreset) => {
    const cam = cameraRef.current;
    const ctrl = controlsRef.current;
    if (!cam || !ctrl) return;
    const cfg = CAMERA_PRESETS[preset];
    cam.position.set(...cfg.pos);
    ctrl.target.set(...cfg.target);
    ctrl.update();
  };

  // Initialize Three.js WebGL + CSS2DRenderer scene once
  useEffect(() => {
    const container = stageRef.current;
    if (!container) return;

    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    } catch (err) {
      setWebglError(err instanceof Error ? err.message : 'WebGL initialization failed');
      return;
    }

    const width = container.clientWidth || 800;
    const height = container.clientHeight || 600;

    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
    renderer.setSize(width, height);
    renderer.domElement.className = 'block w-full h-full';
    container.appendChild(renderer.domElement);

    const labelRenderer = new CSS2DRenderer();
    labelRenderer.setSize(width, height);
    labelRenderer.domElement.style.position = 'absolute';
    labelRenderer.domElement.style.inset = '0';
    labelRenderer.domElement.style.pointerEvents = 'none';
    container.appendChild(labelRenderer.domElement);

    const scene = new THREE.Scene();
    scene.background = new THREE.Color(0x0c0a09);
    scene.fog = new THREE.FogExp2(0x0c0a09, 0.022);

    const camera = new THREE.PerspectiveCamera(44, width / height, 0.1, 160);
    const aspectScale = width < 640 ? 1.48 : 1.0;
    camera.position.set(
      CAMERA_PRESETS.iso.pos[0] * aspectScale,
      CAMERA_PRESETS.iso.pos[1] * aspectScale,
      CAMERA_PRESETS.iso.pos[2] * aspectScale,
    );
    cameraRef.current = camera;

    const controls = new OrbitControls(camera, renderer.domElement);
    controls.enableDamping = true;
    controls.dampingFactor = 0.07;
    controls.target.set(...CAMERA_PRESETS.iso.target);
    controls.minDistance = 8;
    controls.maxDistance = 48;
    controls.maxPolarAngle = Math.PI * 0.85;
    controls.update();
    controlsRef.current = controls;

    // Layer planes + subtle boundary frames
    for (const [layerId, cfg] of Object.entries(LAYER_CONFIG) as Array<
      [LayerId, (typeof LAYER_CONFIG)[LayerId]]
    >) {
      const planeGeom = new THREE.PlaneGeometry(21.5, 11.4);
      const planeMat = new THREE.MeshBasicMaterial({
        color: cfg.hex,
        transparent: true,
        opacity: 0.05,
        side: THREE.DoubleSide,
        depthWrite: false,
      });
      const plane = new THREE.Mesh(planeGeom, planeMat);
      plane.rotation.x = -Math.PI / 2;
      plane.position.set(0, cfg.y - 0.15, 0);
      scene.add(plane);

      const edges = new THREE.EdgesGeometry(planeGeom);
      const border = new THREE.LineSegments(
        edges,
        new THREE.LineBasicMaterial({ color: cfg.hex, transparent: true, opacity: 0.28 }),
      );
      border.rotation.x = -Math.PI / 2;
      border.position.set(0, cfg.y - 0.14, 0);
      scene.add(border);

      const layerTagEl = document.createElement('div');
      layerTagEl.className =
        'px-2 py-0.5 rounded border text-floor font-semibold tracking-wide bg-stone-950/90 text-stone-300 border-white/10 whitespace-nowrap select-none';
      layerTagEl.textContent = layerId.toUpperCase();
      const layerObj = new CSS2DObject(layerTagEl);
      layerObj.position.set(-10.8, cfg.y + 0.05, 5.5);
      scene.add(layerObj);
    }

    // Component node markers + CSS2D interactive cards
    const cardMap = new Map<string, HTMLButtonElement>();
    for (const comp of ARCH.components) {
      const pos = positions.get(comp.id);
      if (!pos) continue;
      const cfg = LAYER_CONFIG[comp.layer];

      const anchorGeom = new THREE.CylinderGeometry(0.18, 0.22, 0.1, 20);
      const anchorMat = new THREE.MeshBasicMaterial({ color: cfg.hex });
      const anchor = new THREE.Mesh(anchorGeom, anchorMat);
      anchor.position.copy(pos).setY(pos.y - 0.1);
      scene.add(anchor);

      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className =
        'pointer-events-auto text-left rounded-xl px-2 py-1 border bg-stone-900/92 backdrop-blur-md transition-all shadow-lg cursor-pointer max-w-[136px] sm:max-w-[164px]';
      btn.innerHTML = `
        <div class="text-floor font-semibold text-stone-100 leading-tight truncate">${comp.label}</div>
        <div class="hidden sm:block text-floor text-stone-400 font-mono truncate opacity-85">${comp.id}</div>
      `;
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        setSelectedComponentId(comp.id);
      });

      const labelObj = new CSS2DObject(btn);
      labelObj.position.set(pos.x, pos.y + 0.42, pos.z);
      scene.add(labelObj);
      cardMap.set(comp.id, btn);
    }
    cardElsRef.current = cardMap;

    // Static dependency curves
    const depLines: Array<{ from: string; to: string; line: THREE.Line }> = [];
    for (const comp of ARCH.components) {
      const src = positions.get(comp.id);
      if (!src) continue;
      for (const depId of comp.dependsOn) {
        const dst = positions.get(depId);
        if (!dst) continue;
        const curve = buildEdgeCurve(src, dst, 0);
        const geom = new THREE.BufferGeometry().setFromPoints(curve.getPoints(28));
        const mat = new THREE.LineBasicMaterial({
          color: 0x78716c,
          transparent: true,
          opacity: 0.3,
        });
        const line = new THREE.Line(geom, mat);
        scene.add(line);
        depLines.push({ from: comp.id, to: depId, line });
      }
    }
    depEdgesRef.current = depLines;

    const flowGroup = new THREE.Group();
    scene.add(flowGroup);
    flowGroupRef.current = flowGroup;

    const onResize = () => {
      if (!container) return;
      const w = container.clientWidth || 800;
      const h = container.clientHeight || 600;
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
      renderer.setSize(w, h);
      labelRenderer.setSize(w, h);
    };
    window.addEventListener('resize', onResize);

    const clock = new THREE.Clock();
    let rafId = 0;
    const animate = () => {
      rafId = requestAnimationFrame(animate);
      const elapsed = clock.getElapsedTime();
      controls.autoRotate = autoRotateRef.current;
      controls.autoRotateSpeed = 0.85;
      controls.update();

      for (const seg of flowSegmentsRef.current) {
        const t = (elapsed * 0.38 + seg.offset) % 1;
        seg.curve.getPointAt(t, seg.orb.position);
      }

      renderer.render(scene, camera);
      labelRenderer.render(scene, camera);
    };
    animate();

    return () => {
      cancelAnimationFrame(rafId);
      window.removeEventListener('resize', onResize);
      controls.dispose();
      renderer.dispose();
      if (renderer.domElement.parentNode === container) container.removeChild(renderer.domElement);
      if (labelRenderer.domElement.parentNode === container) container.removeChild(labelRenderer.domElement);
      cardElsRef.current.clear();
      depEdgesRef.current = [];
      flowSegmentsRef.current = [];
    };
  }, [positions]);

  // Synchronize selection, boundary highlights, and animated pipeline tubes
  useEffect(() => {
    const pipelineComponents = new Set(activePipeline?.steps.map((s) => s.component) ?? []);
    const selectedComp = ARCH.components.find((c) => c.id === selectedComponentId) ?? null;
    const connectedIds = new Set<string>();
    if (selectedComp) {
      connectedIds.add(selectedComp.id);
      for (const d of selectedComp.dependsOn) connectedIds.add(d);
      for (const other of ARCH.components) {
        if (other.dependsOn.includes(selectedComp.id)) connectedIds.add(other.id);
      }
    }

    for (const comp of ARCH.components) {
      const btn = cardElsRef.current.get(comp.id);
      if (!btn) continue;
      const isSelected = comp.id === selectedComponentId;
      const inPipeline = reviewMode === 'pipelines' && pipelineComponents.has(comp.id);
      const isRelated = connectedIds.has(comp.id);
      const layerAccent = LAYER_CONFIG[comp.layer].accentClass;

      btn.className = [
        'pointer-events-auto text-left rounded-xl px-2 py-1 border backdrop-blur-md transition-all cursor-pointer max-w-[136px] sm:max-w-[164px]',
        isSelected
          ? 'bg-rose-950/92 border-rose-400 ring-2 ring-rose-400/50 scale-105 shadow-xl'
          : inPipeline
            ? `bg-stone-900/95 ${layerAccent} shadow-lg`
            : isRelated
              ? 'bg-stone-900/90 border-stone-500/70'
              : 'bg-stone-950/75 border-white/10 opacity-45 hover:opacity-100',
      ].join(' ');
    }

    for (const edge of depEdgesRef.current) {
      const mat = edge.line.material as THREE.LineBasicMaterial;
      const touchesSelected =
        selectedComponentId !== null &&
        (edge.from === selectedComponentId || edge.to === selectedComponentId);
      if (touchesSelected) {
        mat.color.setHex(0xfb7185);
        mat.opacity = 0.9;
      } else if (reviewMode === 'overview') {
        mat.color.setHex(0xa8a29e);
        mat.opacity = 0.45;
      } else {
        mat.color.setHex(0x57534e);
        mat.opacity = 0.18;
      }
    }

    const flowGroup = flowGroupRef.current;
    if (!flowGroup) return;
    while (flowGroup.children.length > 0) {
      const child = flowGroup.children.pop()!;
      if (child instanceof THREE.Mesh) {
        child.geometry.dispose();
        if (Array.isArray(child.material)) {
          child.material.forEach((m) => m.dispose());
        } else {
          child.material.dispose();
        }
      }
    }
    flowSegmentsRef.current = [];

    if (reviewMode !== 'pipelines' || !activePipeline) return;
    const modeColor = MODE_META[activePipeline.mode].hex;
    const steps = activePipeline.steps;
    for (let i = 0; i < steps.length - 1; i++) {
      const src = positions.get(steps[i].component);
      const dst = positions.get(steps[i + 1].component);
      if (!src || !dst) continue;
      const sameNode = steps[i].component === steps[i + 1].component;
      const curve = sameNode
        ? new THREE.CatmullRomCurve3([
            src.clone(),
            src.clone().add(new THREE.Vector3(0.9, 1.0, 0.7)),
            src.clone().add(new THREE.Vector3(-0.9, 1.0, 0.7)),
            dst.clone(),
          ])
        : buildEdgeCurve(src, dst, i + 1);

      const tubeGeom = new THREE.TubeGeometry(curve, 32, 0.045, 8, false);
      const tubeMat = new THREE.MeshBasicMaterial({
        color: modeColor,
        transparent: true,
        opacity: 0.78,
      });
      flowGroup.add(new THREE.Mesh(tubeGeom, tubeMat));

      const orbGeom = new THREE.SphereGeometry(0.14, 14, 14);
      const orbMat = new THREE.MeshBasicMaterial({ color: 0xfafaf9 });
      const orb = new THREE.Mesh(orbGeom, orbMat);
      flowGroup.add(orb);
      flowSegmentsRef.current.push({
        curve,
        orb,
        offset: (i * 0.25) % 1,
      });
    }
  }, [activePipeline, positions, reviewMode, selectedComponentId]);

  const handleSelectPipeline = (pipeline: ArchPipeline) => {
    setSelectedPipelineId(pipeline.id);
    setReviewMode('pipelines');
    const first = pipeline.steps[0];
    if (first) {
      setSelectedStepId(first.id);
      setSelectedComponentId(first.component);
    }
  };

  return (
    <div class="h-[100dvh] w-screen overflow-hidden flex flex-col bg-stone-950 text-stone-100">
      {/* Top Toolbar */}
      <header class="shrink-0 border-b border-white/10 bg-stone-950/90 backdrop-blur-md px-4 py-2.5 flex flex-wrap items-center justify-between gap-2 z-20">
        <div class="flex items-center gap-3 min-w-0">
          <a
            href="/"
            class="px-2.5 py-1 rounded-lg border border-white/10 bg-stone-900 text-stone-300 hover:text-stone-100 text-floor font-medium transition-colors"
          >
            Back
          </a>
          {import.meta.env.DEV && (
            <a
              href="/?view=graph"
              class="px-2.5 py-1 rounded-lg border border-sky-400/40 bg-sky-500/15 text-sky-200 hover:bg-sky-500/25 text-floor font-medium transition-colors"
            >
              2D Graph (Dev)
            </a>
          )}
          <div class="min-w-0">
            <h1 class="text-body font-bold tracking-tight text-stone-100 truncate">
              Architecture & Pipeline Reviewer
            </h1>
            <p class="text-floor text-stone-400 truncate">
              Source: docs/architecture.manifest.json ({ARCH.components.length} components /{' '}
              {ARCH.pipelines.length} pipelines)
            </p>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-1.5">
          {(
            [
              { id: 'pipelines', label: 'Pipelines', icon: GitBranch },
              { id: 'boundaries', label: 'Boundaries', icon: Shield },
              { id: 'overview', label: '3D Dependencies', icon: Layers },
            ] as const
          ).map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setReviewMode(tab.id)}
              class={`px-2.5 py-1 rounded-lg border text-floor font-medium flex items-center gap-1.5 transition-colors cursor-pointer ${
                reviewMode === tab.id
                  ? 'bg-rose-500/20 border-rose-400/50 text-rose-200'
                  : 'bg-stone-900 border-white/10 text-stone-300 hover:border-white/25'
              }`}
            >
              <Icon icon={tab.icon} size={14} />
              <span>{tab.label}</span>
            </button>
          ))}

          <span class="hidden sm:inline-block w-px h-5 bg-white/10 mx-1" />

          <button
            type="button"
            onClick={() => applyCameraPreset('iso')}
            class="px-2 py-1 rounded-lg border border-white/10 bg-stone-900 text-stone-300 hover:text-stone-100 text-floor cursor-pointer"
          >
            Iso
          </button>
          <button
            type="button"
            onClick={() => applyCameraPreset('top')}
            class="px-2 py-1 rounded-lg border border-white/10 bg-stone-900 text-stone-300 hover:text-stone-100 text-floor cursor-pointer"
          >
            Top
          </button>
          <button
            type="button"
            onClick={() => setAutoRotate((v) => !v)}
            class={`px-2 py-1 rounded-lg border text-floor flex items-center gap-1 cursor-pointer ${
              autoRotate
                ? 'bg-emerald-500/15 border-emerald-400/40 text-emerald-200'
                : 'bg-stone-900 border-white/10 text-stone-300'
            }`}
          >
            <Icon icon={RotateCcw} size={13} />
            <span>Orbit</span>
          </button>
        </div>
      </header>

      {/* Main Split Stage */}
      <div class="flex-1 min-h-0 flex flex-col lg:flex-row">
        {/* Left: Three.js 3D Stage */}
        <div class="relative flex-1 min-h-[42dvh] lg:min-h-0 overflow-hidden bg-stone-950">
          <div ref={stageRef} class="w-full h-full relative" />

          {webglError && (
            <div class="absolute inset-4 flex items-center justify-center">
              <div class="soft-card rounded-2xl p-5 max-w-md border border-amber-500/30 text-meta text-stone-200">
                WebGL 3D ビューを初期化できませんでした（{webglError}）。右側のインスペクタからパイプラインと境界を確認できます。
              </div>
            </div>
          )}

          {/* Legend overlay */}
          <div class="pointer-events-none absolute left-3 bottom-3 flex flex-wrap gap-1.5 z-10">
            {(Object.entries(LAYER_CONFIG) as Array<[LayerId, (typeof LAYER_CONFIG)[LayerId]]>).map(
              ([id, cfg]) => (
                <span
                  key={id}
                  class={`px-2 py-0.5 rounded-md border text-floor font-medium ${cfg.badgeClass}`}
                >
                  {cfg.title}
                </span>
              ),
            )}
          </div>
        </div>

        {/* Right: Structured Pipeline & Boundary Inspector */}
        <aside class="w-full lg:w-[440px] xl:w-[480px] shrink-0 border-t lg:border-t-0 lg:border-l border-white/10 bg-stone-950/95 overflow-y-auto p-3.5 space-y-3.5">
          {reviewMode === 'pipelines' && (
            <section class="space-y-3">
              <div class="flex flex-wrap gap-1.5">
                {ARCH.pipelines.map((p) => {
                  const isActive = p.id === selectedPipelineId;
                  const meta = MODE_META[p.mode];
                  return (
                    <button
                      key={p.id}
                      type="button"
                      onClick={() => handleSelectPipeline(p)}
                      class={`px-2.5 py-1.5 rounded-xl border text-left transition-all cursor-pointer ${
                        isActive
                          ? 'bg-stone-900 border-rose-400/60 text-stone-100 shadow-md'
                          : 'bg-stone-900/50 border-white/10 text-stone-400 hover:text-stone-200'
                      }`}
                    >
                      <div class="text-floor font-semibold leading-tight">{p.name}</div>
                      <div class={`inline-block mt-1 px-1.5 py-0.2 rounded border text-floor ${meta.pillClass}`}>
                        {meta.label}
                      </div>
                    </button>
                  );
                })}
              </div>

              {activePipeline && (
                <div class="soft-card rounded-2xl p-3.5 space-y-3">
                  <div>
                    <h2 class="text-body font-bold text-stone-100">{activePipeline.name}</h2>
                    <p class="text-meta text-stone-400 mt-0.5 leading-relaxed">
                      {activePipeline.description}
                    </p>
                  </div>

                  <div class="space-y-2.5">
                    {activePipeline.steps.map((step, idx) => {
                      const isStepActive =
                        selectedStepId === step.id || selectedComponentId === step.component;
                      return (
                        <button
                          key={step.id}
                          type="button"
                          onClick={() => {
                            setSelectedStepId(step.id);
                            setSelectedComponentId(step.component);
                          }}
                          class={`w-full text-left rounded-xl p-3 border transition-all cursor-pointer ${
                            isStepActive
                              ? 'bg-stone-900 border-rose-400/50'
                              : 'bg-stone-950/60 border-white/10 hover:border-white/25'
                          }`}
                        >
                          <div class="flex items-center justify-between gap-2">
                            <span class="text-meta font-semibold text-stone-100">
                              {idx + 1}. {step.label}
                            </span>
                            <span class="px-2 py-0.5 rounded-md bg-stone-800 border border-white/10 text-floor font-mono text-stone-300">
                              {step.component}
                            </span>
                          </div>
                          <div class="text-floor font-mono text-stone-400 mt-1 truncate">
                            {step.entryPath}
                          </div>
                          <p class="text-floor text-stone-300 mt-1.5 leading-snug">{step.summary}</p>
                          <div class="flex flex-wrap gap-1.5 mt-2">
                            {step.guards.map((g) => (
                              <span
                                key={g}
                                class="px-2 py-0.5 rounded-md bg-emerald-500/12 border border-emerald-500/30 text-emerald-300 text-floor"
                              >
                                {g}
                              </span>
                            ))}
                            {step.stores?.map((st) => (
                              <span
                                key={st}
                                class="px-2 py-0.5 rounded-md bg-sky-500/12 border border-sky-500/30 text-sky-300 text-floor inline-flex items-center gap-1"
                              >
                                <Icon icon={Database} size={12} />
                                {st}
                              </span>
                            ))}
                          </div>
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}
            </section>
          )}

          {reviewMode === 'boundaries' && (
            <section class="space-y-3">
              <div class="soft-card rounded-2xl p-3.5 space-y-2.5">
                <h2 class="text-body font-bold text-stone-100 flex items-center gap-2">
                  <Icon icon={Shield} size={16} />
                  <span>テナント分離境界（{ARCH.boundaries.length} layers）</span>
                </h2>
                <div class="space-y-2">
                  {ARCH.boundaries.map((b) => {
                    const st = BOUNDARY_META[b.level];
                    return (
                      <div
                        key={b.layer}
                        class="rounded-xl p-2.5 border border-white/10 bg-stone-950/70 space-y-1"
                      >
                        <div class="flex items-center justify-between gap-2">
                          <span class="text-meta font-semibold text-stone-100 font-mono">
                            {b.layer}
                          </span>
                          <span class={`px-2 py-0.5 rounded-md border text-floor ${st.pillClass}`}>
                            {st.label}
                          </span>
                        </div>
                        <p class="text-floor text-stone-300 leading-snug">{b.details}</p>
                        <div class="text-floor font-mono text-stone-500 truncate">{b.path}</div>
                      </div>
                    );
                  })}
                </div>
              </div>

              <div class="soft-card rounded-2xl p-3.5 space-y-2">
                <h3 class="text-meta font-bold text-stone-100">
                  登録テナント（{ARCH.tenants.length}）
                </h3>
                {ARCH.tenants.map((t) => (
                  <div key={t.id} class="rounded-xl p-2.5 border border-white/10 bg-stone-950/70">
                    <div class="flex items-center justify-between">
                      <span class="text-meta font-bold text-rose-300">{t.id}</span>
                      <span class="text-floor font-mono text-stone-400">{t.storagePrefix}*</span>
                    </div>
                    <div class="text-floor text-stone-300 mt-1">{t.hosts.join(', ')}</div>
                    <div class="text-floor font-mono text-stone-500 mt-0.5 truncate">
                      {t.dataUrl} (fallback: {t.fallbackDataUrl})
                    </div>
                  </div>
                ))}
              </div>
            </section>
          )}

          {/* Selected Component Details Card (Always accessible) */}
          {selectedComponent && (
            <section class="soft-card rounded-2xl p-3.5 space-y-2.5">
              <div class="flex items-center justify-between gap-2">
                <div class="flex items-center gap-2 min-w-0">
                  <Icon icon={Server} size={16} class="text-rose-400 shrink-0" />
                  <h3 class="text-meta font-bold text-stone-100 truncate">
                    {selectedComponent.label}
                  </h3>
                </div>
                <span
                  class={`px-2 py-0.5 rounded-md border text-floor shrink-0 ${
                    LAYER_CONFIG[selectedComponent.layer].badgeClass
                  }`}
                >
                  {selectedComponent.layer}
                </span>
              </div>

              <div class="text-floor font-mono text-stone-300 bg-stone-950/80 px-2.5 py-1.5 rounded-lg border border-white/10 truncate">
                {selectedComponent.path}
              </div>

              <div>
                <div class="text-floor text-stone-400 mb-1">依存先 (dependsOn)</div>
                {selectedComponent.dependsOn.length === 0 ? (
                  <span class="text-floor text-stone-500">なし（終端 / 独立コンポーネント）</span>
                ) : (
                  <div class="flex flex-wrap gap-1.5">
                    {selectedComponent.dependsOn.map((depId) => (
                      <button
                        key={depId}
                        type="button"
                        onClick={() => setSelectedComponentId(depId)}
                        class="px-2 py-0.5 rounded-md border border-white/10 bg-stone-900 hover:border-rose-400/50 text-floor text-stone-200 inline-flex items-center gap-1 cursor-pointer"
                      >
                        <Icon icon={ArrowRight} size={12} />
                        {depId}
                      </button>
                    ))}
                  </div>
                )}
              </div>

              {relatedSteps.length > 0 && (
                <div>
                  <div class="text-floor text-stone-400 mb-1">関連パイプラインステップ</div>
                  <div class="space-y-1">
                    {relatedSteps.map(({ pipeline, step, index }) => (
                      <button
                        key={`${pipeline.id}-${step.id}`}
                        type="button"
                        onClick={() => {
                          setReviewMode('pipelines');
                          setSelectedPipelineId(pipeline.id);
                          setSelectedStepId(step.id);
                        }}
                        class="w-full text-left px-2.5 py-1.5 rounded-lg border border-white/10 bg-stone-950/70 hover:border-rose-400/40 text-floor text-stone-300 flex items-center justify-between gap-2 cursor-pointer"
                      >
                        <span class="truncate">
                          [{pipeline.id}] #{index} {step.label}
                        </span>
                        <Icon icon={Box} size={13} class="shrink-0 text-stone-400" />
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </section>
          )}
        </aside>
      </div>
    </div>
  );
}
