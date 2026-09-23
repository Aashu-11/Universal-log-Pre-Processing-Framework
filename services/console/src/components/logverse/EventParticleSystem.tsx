import { useMemo, useRef } from "react";
import { useFrame } from "@react-three/fiber";
import { Instance, Instances } from "@react-three/drei";
import type { Group, Object3D } from "three";

import { hasArrived, progressForEvent } from "../../lib/logverse/playback";
import { positionForEvent, vendorSpawnPosition, type Vec3 } from "../../lib/logverse/layout";
import type { EventVisualState, LogVerseEvent } from "../../lib/logverse/types";

const STATE_COLOR: Record<EventVisualState, string> = {
  normal: "#22d3ee",
  elevated: "#f6be4f",
  threat: "#fb5b71",
  dlq: "#b06bf0",
};

function partitionByState(events: LogVerseEvent[]): Record<EventVisualState, LogVerseEvent[]> {
  const out: Record<EventVisualState, LogVerseEvent[]> = { normal: [], elevated: [], threat: [], dlq: [] };
  for (const e of events) out[e.visualState].push(e);
  return out;
}

interface ParticleProps {
  event: LogVerseEvent;
  vendorPositions: Map<string, Vec3>;
  clockMsRef: React.RefObject<number>;
  onSelect: (ev: LogVerseEvent) => void;
  onHover: (ev: LogVerseEvent | null, worldPos: Vec3 | null) => void;
}

/** One particle. Deliberately cheap: a ref + a useFrame callback that
 * mutates the underlying three.js object directly — never touches React
 * state, so having up to MAX_VISIBLE_PARTICLES of these costs zero
 * reconciliation, only a per-frame position write (the R3F-recommended
 * pattern for animated instanced content, and exactly what "don't create
 * one expensive React component per event" is warning against avoiding
 * the *cheap* version of). Selection highlighting is handled by the
 * separate SelectionHighlighter overlay below, not per-particle state, so
 * changing the selection never touches these. */
function Particle({ event, vendorPositions, clockMsRef, onSelect, onHover }: ParticleProps) {
  const ref = useRef<Object3D>(null);
  const sourcePos = useMemo(() => vendorSpawnPosition(event.vendor, vendorPositions), [event.vendor, vendorPositions]);

  useFrame(() => {
    if (!ref.current) return;
    const clock = clockMsRef.current ?? Date.now();
    const progress = progressForEvent(event, clock);
    const [x, y, z] = positionForEvent(event, sourcePos, progress);
    ref.current.position.set(x, y, z);
    ref.current.visible = !hasArrived(event, clock);
  });

  return (
    <Instance
      ref={ref as React.Ref<Object3D>}
      color={STATE_COLOR[event.visualState]}
      onClick={(e) => {
        e.stopPropagation();
        onSelect(event);
      }}
      onPointerOver={(e) => {
        e.stopPropagation();
        onHover(event, [e.point.x, e.point.y, e.point.z]);
      }}
      onPointerOut={() => onHover(null, null)}
    />
  );
}

const GEOMETRY_LIMIT = 200; // headroom above MAX_VISIBLE_PARTICLES so a burst doesn't need remounting

function ParticleGroup({
  events,
  geometry,
  vendorPositions,
  clockMsRef,
  onSelect,
  onHover,
}: {
  events: LogVerseEvent[];
  geometry: "sphere" | "octahedron" | "box";
  vendorPositions: Map<string, Vec3>;
  clockMsRef: React.RefObject<number>;
  onSelect: (ev: LogVerseEvent) => void;
  onHover: (ev: LogVerseEvent | null, worldPos: Vec3 | null) => void;
}) {
  if (events.length === 0) return null;
  return (
    <Instances limit={GEOMETRY_LIMIT} range={GEOMETRY_LIMIT}>
      {geometry === "sphere" && <sphereGeometry args={[0.28, 12, 12]} />}
      {geometry === "octahedron" && <octahedronGeometry args={[0.36, 0]} />}
      {geometry === "box" && <boxGeometry args={[0.42, 0.42, 0.42]} />}
      <meshStandardMaterial emissiveIntensity={0.6} toneMapped={false} />
      {events.map((ev) => (
        <Particle key={ev.eventId} event={ev} vendorPositions={vendorPositions} clockMsRef={clockMsRef} onSelect={onSelect} onHover={onHover} />
      ))}
    </Instances>
  );
}

/** Renders the currently-visible (already filtered + capped by the caller
 * via selectVisibleParticles) events as instanced particles, one
 * <Instances> group per visual state so each state gets a genuinely
 * different shape (spec: not color alone) while staying within a small,
 * fixed number of draw calls regardless of how many particles are active. */
export function EventParticleSystem({
  events,
  vendorPositions,
  clockMsRef,
  selectedId,
  onSelect,
  onHover,
}: {
  events: LogVerseEvent[];
  vendorPositions: Map<string, Vec3>;
  clockMsRef: React.RefObject<number>;
  selectedId: string | null;
  onSelect: (ev: LogVerseEvent) => void;
  onHover: (ev: LogVerseEvent | null, worldPos: Vec3 | null) => void;
}) {
  const grouped = useMemo(() => partitionByState(events), [events]);
  const shared = { vendorPositions, clockMsRef, onSelect, onHover };

  return (
    <group>
      <ParticleGroup events={grouped.normal} geometry="sphere" {...shared} />
      <ParticleGroup events={grouped.elevated} geometry="sphere" {...shared} />
      <ParticleGroup events={grouped.threat} geometry="octahedron" {...shared} />
      <ParticleGroup events={grouped.dlq} geometry="box" {...shared} />
      <SelectionHighlighter events={events} selectedId={selectedId} vendorPositions={vendorPositions} clockMsRef={clockMsRef} />
    </group>
  );
}

/** A single extra marker (not an Instance, since there's at most one
 * selection) that tracks the selected particle's live position — a white
 * ring drawn around whichever particle is selected, distinct from every
 * color used elsewhere in the legend. Also stays visible past the
 * particle's normal "arrived" hide, since an investigator selecting an
 * event wants to keep seeing where it ended up while inspecting it. */
function SelectionHighlighter({
  events,
  selectedId,
  vendorPositions,
  clockMsRef,
}: {
  events: LogVerseEvent[];
  selectedId: string | null;
  vendorPositions: Map<string, Vec3>;
  clockMsRef: React.RefObject<number>;
}) {
  const ref = useRef<Group>(null);
  const selected = selectedId ? (events.find((e) => e.eventId === selectedId) ?? null) : null;
  const sourcePos = selected ? vendorSpawnPosition(selected.vendor, vendorPositions) : null;

  useFrame(() => {
    if (!ref.current || !selected || !sourcePos) return;
    const clock = clockMsRef.current ?? Date.now();
    const progress = progressForEvent(selected, clock);
    const [x, y, z] = positionForEvent(selected, sourcePos, Math.min(progress, 1));
    ref.current.position.set(x, y, z);
  });

  if (!selected) return null;
  return (
    <group ref={ref}>
      <mesh rotation={[Math.PI / 2, 0, 0]}>
        <ringGeometry args={[0.55, 0.68, 24]} />
        <meshBasicMaterial color="#ffffff" transparent opacity={0.85} />
      </mesh>
    </group>
  );
}
