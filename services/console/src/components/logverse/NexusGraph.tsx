import { forwardRef, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import ForceGraph3D, { type ForceGraphMethods } from "react-force-graph-3d";

import { buildNexus, type NexusNode } from "../../lib/logverse/nexus";
import type { LogVerseEvent } from "../../lib/logverse/types";

const colors = { normal: "#22d3ee", elevated: "#f6be4f", threat: "#fb5b71", dlq: "#b06bf0" };
type GraphNode = NexusNode & { x: number; y: number; z: number; val: number };
type GraphLink = { source: string | GraphNode; target: string | GraphNode; state: LogVerseEvent["visualState"] };

export interface NexusGraphHandle { resetCamera: () => void }

function endpointId(endpoint: string | GraphNode): string { return typeof endpoint === "string" ? endpoint : endpoint.id; }

/** The supplied LedgerSpy graph interaction, mapped to real LOGKRAMA events. */
export const NexusGraph = forwardRef<NexusGraphHandle, {
  events: LogVerseEvent[];
  selectedId: string | null;
  onSelect: (event: LogVerseEvent) => void;
  onNodeSelect: (node: NexusNode) => void;
  reducedMotion: boolean;
  paused: boolean;
}>(function NexusGraph({ events, selectedId, onSelect, onNodeSelect, reducedMotion, paused }, ref) {
  const containerRef = useRef<HTMLDivElement>(null);
  const graphRef = useRef<ForceGraphMethods<GraphNode, GraphLink> | undefined>(undefined);
  const [size, setSize] = useState({ width: 800, height: 600 });
  const [hoveredId, setHoveredId] = useState<string | null>(null);
  const cameraInitialized = useRef(false);

  const graph = useMemo(() => {
    const source = buildNexus(events);
    return {
      nodes: source.nodes.map((node): GraphNode => ({ ...node, x: node.position[0] * 2, y: node.position[1] * 2, z: node.position[2] * 2, val: node.kind === "vendor" ? 10 : node.kind === "reason" ? 8 : node.kind === "endpoint" ? 6 : 3 })),
      links: source.links.map((link): GraphLink => ({ ...link })),
    };
  }, [events]);

  const highlighted = useMemo(() => {
    if (!hoveredId) return null;
    const ids = new Set([hoveredId]);
    for (const link of graph.links) {
      const source = endpointId(link.source);
      const target = endpointId(link.target);
      if (source === hoveredId) ids.add(target);
      if (target === hoveredId) ids.add(source);
    }
    return ids;
  }, [graph, hoveredId]);

  useEffect(() => {
    const element = containerRef.current;
    if (!element) return;
    const observer = new ResizeObserver(([entry]) => {
      if (!entry) return;
      const rect = entry.contentRect;
      setSize({ width: Math.max(1, Math.floor(rect.width)), height: Math.max(1, Math.floor(rect.height)) });
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (paused) graphRef.current?.pauseAnimation();
    else graphRef.current?.resumeAnimation();
  }, [paused]);

  useEffect(() => {
    if (cameraInitialized.current || graph.nodes.length === 0) return;
    graphRef.current?.cameraPosition({ x: 0, y: 0, z: 160 }, { x: 0, y: 0, z: 0 }, 0);
  }, [graph]);

  useImperativeHandle(ref, () => ({ resetCamera: () => graphRef.current?.zoomToFit(reducedMotion ? 0 : 900, 120) }), [reducedMotion]);

  const colorFor = (node: GraphNode) => node.kind === "vendor" ? "#8b5cf6" : node.kind === "reason" ? "#b06bf0" : node.kind === "endpoint" ? "#3b82f6" : colors[node.event?.visualState ?? "normal"];
  const flyTo = (node: GraphNode) => {
    const ratio = 1 + 40 / Math.max(1, Math.hypot(node.x, node.y, node.z));
    graphRef.current?.cameraPosition({ x: node.x * ratio, y: node.y * ratio, z: node.z * ratio }, { x: node.x, y: node.y, z: node.z }, reducedMotion ? 0 : 3000);
    onNodeSelect(node);
    if (node.event) onSelect(node.event);
  };

  return <div ref={containerRef} className="h-full w-full" aria-label="3D force-directed log relationship graph">
    <ForceGraph3D<GraphNode, GraphLink>
      ref={graphRef}
      graphData={graph}
      width={size.width}
      height={size.height}
      backgroundColor="#050506"
      showNavInfo={false}
      nodeLabel={(node) => `${node.label}${node.event ? ` · ${node.event.visualState}` : ""}`}
      nodeColor={(node) => highlighted === null || highlighted.has(node.id) ? colorFor(node) : "#15151e"}
      nodeVal={(node) => node.val * (node.event?.eventId === selectedId ? 1.7 : 1)}
      nodeRelSize={3.5}
      nodeResolution={16}
      linkColor={(link) => highlighted && !(highlighted.has(endpointId(link.source)) && highlighted.has(endpointId(link.target))) ? "#0b0b0f" : colors[link.state]}
      linkOpacity={0.34}
      linkWidth={(link) => highlighted?.has(endpointId(link.source)) && highlighted.has(endpointId(link.target)) ? 2 : 0.7}
      linkDirectionalParticles={(link) => reducedMotion || (highlighted !== null && !(highlighted.has(endpointId(link.source)) && highlighted.has(endpointId(link.target)))) ? 0 : 2}
      linkDirectionalParticleWidth={2}
      linkDirectionalParticleSpeed={0.007}
      linkDirectionalParticleColor={(link) => colors[link.state]}
      onNodeHover={(node) => setHoveredId(node?.id ?? null)}
      onNodeClick={flyTo}
      onEngineStop={() => {
        if (cameraInitialized.current) return;
        cameraInitialized.current = true;
        graphRef.current?.zoomToFit(reducedMotion ? 0 : 1100, 120);
      }}
      enableNodeDrag={false}
      cooldownTicks={120}
      warmupTicks={45}
      d3AlphaDecay={0.025}
    />
  </div>;
});
