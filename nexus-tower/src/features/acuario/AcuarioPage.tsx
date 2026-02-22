import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import { Card } from '../../components/Card';
import { QueryError } from '../../components/QueryError';
import {
  createWorldRun,
  getAuditEvents,
  getWorldEvents,
  getWorldRuns,
  getWorldState,
  replayWorldRun,
} from '../../lib/api';
import type { AuditEventItem, WorldAgentState, WorldEventItem, WorldState } from '../../lib/types';

type OverlayToggles = {
  policy: boolean;
  rate: boolean;
  collision: boolean;
  loop: boolean;
  intention: boolean;
};

type AgentSignals = {
  policy: Map<string, string>;
  rate: Map<string, string>;
  collision: Set<string>;
  collisionSeq: Map<string, number>;
  loops: Set<string>;
  heatmap: Array<{ cell: string; hits: number }>;
  worldFeed: Array<{ key: string; at: string; label: string; detail: string }>;
  enforcementFeed: Array<{ key: string; at: string; label: string; detail: string }>;
};

type IncidentMetrics = {
  moved: number;
  collided: number;
  blocked: number;
  collisions: number;
  denied: number;
  rateLimited: number;
  loops: number;
  jamIndex: number;
  throughput: number;
  severity: 'low' | 'medium' | 'high' | 'critical';
};

type ImpactedAgent = {
  agentID: string;
  moved: number;
  collisions: number;
  denied: number;
  rateLimited: number;
  score: number;
};

function emptySignals(): AgentSignals {
  return {
    policy: new Map(),
    rate: new Map(),
    collision: new Set(),
    collisionSeq: new Map(),
    loops: new Set(),
    heatmap: [],
    worldFeed: [],
    enforcementFeed: [],
  };
}

function severityFromJam(jamIndex: number): IncidentMetrics['severity'] {
  if (jamIndex >= 2.5) return 'critical';
  if (jamIndex >= 1.2) return 'high';
  if (jamIndex >= 0.6) return 'medium';
  return 'low';
}

function computeIncidentMetrics(worldEvents: WorldEventItem[], signals: AgentSignals, maxStep: number): IncidentMetrics {
  let moved = 0;
  let collided = 0;
  let blocked = 0;
  for (const ev of worldEvents) {
    const t = toEventType(ev);
    if (t === 'agent.moved') moved += 1;
    if (t === 'agent.collided') collided += 1;
    if (t === 'agent.blocked') blocked += 1;
  }
  let denied = 0;
  let rateLimited = 0;
  for (const ev of signals.enforcementFeed) {
    if (ev.label === 'tool.denied') denied += 1;
    if (ev.label === 'tool.rate_limited') rateLimited += 1;
  }
  const collisions = collided + blocked;
  const jamIndex = Number((collisions / Math.max(1, moved)).toFixed(2));
  const throughput = Number((moved / Math.max(1, maxStep)).toFixed(2));
  return {
    moved,
    collided,
    blocked,
    collisions,
    denied,
    rateLimited,
    loops: signals.loops.size,
    jamIndex,
    throughput,
    severity: severityFromJam(jamIndex),
  };
}

function computeTopImpactedAgents(worldEvents: WorldEventItem[], auditEvents: AuditEventItem[], runID: string): ImpactedAgent[] {
  const byAgent = new Map<string, ImpactedAgent>();
  const getOrCreate = (agentID: string) => {
    const key = agentID.trim();
    const cur = byAgent.get(key);
    if (cur) {
      return cur;
    }
    const next: ImpactedAgent = { agentID: key, moved: 0, collisions: 0, denied: 0, rateLimited: 0, score: 0 };
    byAgent.set(key, next);
    return next;
  };

  for (const ev of worldEvents) {
    const agentID = (ev.agent_id || '').trim();
    if (agentID === '') continue;
    const t = toEventType(ev);
    const row = getOrCreate(agentID);
    if (t === 'agent.moved') row.moved += 1;
    if (t === 'agent.collided' || t === 'agent.blocked') row.collisions += 1;
  }

  for (const ev of auditEvents) {
    const input = ev.input || {};
    const inputRunID = String(input.run_id || '');
    if (inputRunID !== runID) continue;
    const agentID = String(input.agent_id || '').trim();
    if (agentID === '') continue;
    const row = getOrCreate(agentID);
    const code = ev.error?.code || '';
    if (ev.status === 'blocked' && code === 'POLICY_DENIED') row.denied += 1;
    if (ev.status === 'blocked' && code === 'RATE_LIMITED') row.rateLimited += 1;
  }

  return Array.from(byAgent.values())
    .map((row) => ({
      ...row,
      score: row.collisions*2 + row.denied*3 + row.rateLimited*2 + Math.max(0, 12-row.moved),
    }))
    .sort((a, b) => b.score - a.score)
    .slice(0, 8);
}

function deltaLabel(current: number, base: number): string {
  const delta = current - base;
  if (delta === 0) return '0';
  const sign = delta > 0 ? '+' : '';
  return `${sign}${delta}`;
}

function eventTone(label: string): string {
  if (label === 'tool.denied') return 'denied';
  if (label === 'tool.rate_limited') return 'rate';
  if (label === 'agent.collided' || label === 'agent.blocked') return 'collision';
  if (label === 'agent.moved') return 'moved';
  if (label === 'world.snapshot') return 'snapshot';
  return 'default';
}

function toEventType(item: WorldEventItem): string {
  const payloadType = item.payload?.event_type;
  if (typeof payloadType === 'string' && payloadType !== '') {
    return payloadType;
  }
  return item.tool_name;
}

function buildSignals(worldEvents: WorldEventItem[], auditEvents: AuditEventItem[], runID: string): AgentSignals {
  const policy = new Map<string, string>();
  const rate = new Map<string, string>();
  const collision = new Set<string>();
  const collisionSeq = new Map<string, number>();
  const loops = new Set<string>();
  const worldFeed: Array<{ key: string; at: string; label: string; detail: string }> = [];
  const enforcementFeed: Array<{ key: string; at: string; label: string; detail: string }> = [];

  const heat = new Map<string, number>();
  const perAgentTrace = new Map<string, string[]>();

  for (const ev of worldEvents) {
    const eventType = toEventType(ev);
    const agentID = (ev.agent_id || '').trim();
    if (eventType === 'agent.collided' || eventType === 'agent.blocked') {
      if (agentID !== '') {
        collision.add(agentID);
        collisionSeq.set(agentID, ev.seq);
      }
      const result = ev.payload?.result;
      if (result && typeof result === 'object') {
        const newState = (result as Record<string, unknown>).new_state;
        if (newState && typeof newState === 'object') {
          const x = Number((newState as Record<string, unknown>).x);
          const y = Number((newState as Record<string, unknown>).y);
          if (Number.isFinite(x) && Number.isFinite(y)) {
            const key = `${Math.floor(x)}:${Math.floor(y)}`;
            heat.set(key, (heat.get(key) || 0) + 1);
          }
        }
      }
    }
    if (agentID !== '' && eventType.startsWith('agent.')) {
      const trace = perAgentTrace.get(agentID) || [];
      trace.push(eventType);
      if (trace.length >= 4) {
        const n = trace.length;
        if (trace[n - 1] === trace[n - 3] && trace[n - 2] === trace[n - 4]) {
          loops.add(agentID);
        }
      }
      perAgentTrace.set(agentID, trace.slice(-16));
    }

    worldFeed.push({
      key: `w-${ev.seq}`,
      at: ev.created_at,
      label: eventType,
      detail: `${agentID || '-'} step=${ev.step_id} seq=${ev.seq}`,
    });
  }

  for (const ev of auditEvents) {
    const input = ev.input || {};
    const inputRunID = String(input.run_id || '');
    if (inputRunID !== runID) {
      continue
    }
    const agentID = String(input.agent_id || '').trim();
    const code = ev.error?.code || '';
    if (ev.status === 'blocked' && code === 'POLICY_DENIED' && agentID !== '') {
      policy.set(agentID, `${ev.policy_id || 'policy?'} · ${ev.tool_name}`);
      enforcementFeed.push({
        key: `a-${ev.request_id}-deny`,
        at: ev.created_at,
        label: 'tool.denied',
        detail: `${agentID} ${ev.policy_id || ''}`.trim(),
      });
    }
    if (ev.status === 'blocked' && code === 'RATE_LIMITED' && agentID !== '') {
      rate.set(agentID, `org+tool limit · ${ev.tool_name}`);
      enforcementFeed.push({
        key: `a-${ev.request_id}-rl`,
        at: ev.created_at,
        label: 'tool.rate_limited',
        detail: `${agentID} ${ev.tool_name}`,
      });
    }
  }

  const heatmap = Array.from(heat.entries())
    .map(([cell, hits]) => ({ cell, hits }))
    .sort((a, b) => b.hits - a.hits)
    .slice(0, 10);

  worldFeed.sort((a, b) => (a.at < b.at ? 1 : -1));
  enforcementFeed.sort((a, b) => (a.at < b.at ? 1 : -1));

  return { policy, rate, collision, collisionSeq, loops, heatmap, worldFeed, enforcementFeed };
}

function drawWorld(
  canvas: HTMLCanvasElement,
  worldState: WorldState | undefined,
  selectedAgentID: string,
  signals: AgentSignals,
  toggles: OverlayToggles,
) {
  const ctx = canvas.getContext('2d');
  if (!ctx) {
    return;
  }
  const rect = canvas.getBoundingClientRect();
  const dpr = window.devicePixelRatio || 1;
  canvas.width = Math.max(1, Math.floor(rect.width * dpr));
  canvas.height = Math.max(1, Math.floor(rect.height * dpr));
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, rect.width, rect.height);

  if (!worldState || !worldState.config) {
    ctx.fillStyle = '#5b6a64';
    ctx.font = "14px 'IBM Plex Sans'";
    ctx.fillText('Waiting world state...', 20, 28);
    return;
  }

  const cfg = worldState.config;
  const mapSpan = cfg.width + cfg.height;
  const fitByWidth = ((rect.width - 24) * 2) / Math.max(1, mapSpan);
  const fitByHeight = (rect.height - 56) / Math.max(1, mapSpan*0.52*0.5);
  const tileW = Math.max(10, Math.min(22, Math.min(fitByWidth, fitByHeight) * 1.18));
  const tileH = tileW * 0.52;
  const originX = rect.width * 0.5;
  const originY = 30;
  const wallHeight = 16;

  const iso = (x: number, y: number, z = 0) => ({
    x: originX + (x - y) * tileW * 0.5,
    y: originY + (x + y) * tileH * 0.5 - z,
  });

  const c1 = iso(0, 0);
  const c2 = iso(cfg.width, 0);
  const c3 = iso(cfg.width, cfg.height);
  const c4 = iso(0, cfg.height);

  const floorGrad = ctx.createLinearGradient(0, 0, 0, rect.height);
  floorGrad.addColorStop(0, '#f3efe9');
  floorGrad.addColorStop(1, '#ddd3c5');
  ctx.fillStyle = floorGrad;
  ctx.beginPath();
  ctx.moveTo(c1.x, c1.y);
  ctx.lineTo(c2.x, c2.y);
  ctx.lineTo(c3.x, c3.y);
  ctx.lineTo(c4.x, c4.y);
  ctx.closePath();
  ctx.fill();

  ctx.strokeStyle = 'rgba(60,50,40,0.18)';
  ctx.lineWidth = 1;
  for (let x = 0; x <= cfg.width; x += 4) {
    const s = iso(x, 0);
    const e = iso(x, cfg.height);
    ctx.beginPath();
    ctx.moveTo(s.x, s.y);
    ctx.lineTo(e.x, e.y);
    ctx.stroke();
  }
  for (let y = 0; y <= cfg.height; y += 4) {
    const s = iso(0, y);
    const e = iso(cfg.width, y);
    ctx.beginPath();
    ctx.moveTo(s.x, s.y);
    ctx.lineTo(e.x, e.y);
    ctx.stroke();
  }

  const drawWallSlice = (minY: number, maxY: number) => {
    if (maxY <= minY) {
      return;
    }
    const a = iso(cfg.door_x, minY);
    const b = iso(cfg.door_x, maxY);
    ctx.strokeStyle = '#3d3a34';
    ctx.lineWidth = 4;
    ctx.beginPath();
    ctx.moveTo(a.x, a.y - wallHeight);
    ctx.lineTo(b.x, b.y - wallHeight);
    ctx.stroke();
    ctx.strokeStyle = '#8c8476';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(a.x, a.y);
    ctx.lineTo(b.x, b.y);
    ctx.stroke();
  };
  drawWallSlice(0, cfg.door_min_y);
  drawWallSlice(cfg.door_max_y, cfg.height);

  const agents = [...(worldState.agents || [])].sort((a, b) => (a.x + a.y) - (b.x + b.y));
  let latestSeq = 0;
  for (const item of signals.worldFeed) {
    const m = item.detail.match(/seq=(\d+)/);
    if (m && Number(m[1]) > latestSeq) {
      latestSeq = Number(m[1]);
    }
  }
  for (const ag of agents) {
    const p = iso(ag.x, ag.y, 3);
    let color = '#136f63';
    if (toggles.policy && signals.policy.has(ag.id)) {
      color = '#d7263d';
    } else if (toggles.rate && signals.rate.has(ag.id)) {
      color = '#eab308';
    } else if (toggles.collision && signals.collision.has(ag.id)) {
      color = '#fb923c';
    } else if (toggles.loop && signals.loops.has(ag.id)) {
      color = '#d100ff';
    }

    ctx.fillStyle = 'rgba(0,0,0,0.22)';
    ctx.beginPath();
    ctx.ellipse(p.x, p.y + 6, 8, 3.5, 0, 0, Math.PI * 2);
    ctx.fill();

    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.arc(p.x, p.y, 7.6, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = 'rgba(25, 28, 27, 0.35)';
    ctx.lineWidth = 1.1;
    ctx.stroke();

    if (toggles.collision && signals.collision.has(ag.id)) {
      const seq = signals.collisionSeq.get(ag.id) || 0;
      const recent = latestSeq > 0 && latestSeq-seq <= 18;
      const ringR = recent ? 15 : 12;
      ctx.strokeStyle = recent ? 'rgba(251,146,60,0.95)' : 'rgba(251,146,60,0.55)';
      ctx.lineWidth = recent ? 3.1 : 1.7;
      ctx.beginPath();
      ctx.arc(p.x, p.y, ringR, 0, Math.PI * 2);
      ctx.stroke();
    }

    if (ag.id === selectedAgentID) {
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 2.4;
      ctx.beginPath();
      ctx.arc(p.x, p.y, 10.8, 0, Math.PI * 2);
      ctx.stroke();
    }

    if (Math.abs(ag.vx) + Math.abs(ag.vy) > 0.0001) {
      const tail = iso(ag.x - ag.vx*1.8, ag.y - ag.vy*1.8, 2.8);
      ctx.strokeStyle = 'rgba(9, 82, 186, 0.35)';
      ctx.lineWidth = 1.4;
      ctx.beginPath();
      ctx.moveTo(p.x, p.y);
      ctx.lineTo(tail.x, tail.y);
      ctx.stroke();
    }

    if (toggles.intention && Number.isFinite(ag.intention_x) && Number.isFinite(ag.intention_y)) {
      const tip = iso(Number(ag.intention_x), Number(ag.intention_y), 3);
      ctx.strokeStyle = 'rgba(13,110,253,0.75)';
      ctx.lineWidth = 1.2;
      ctx.beginPath();
      ctx.moveTo(p.x, p.y);
      ctx.lineTo(tip.x, tip.y);
      ctx.stroke();
    }
  }
}

export function AcuarioPage() {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const compareCanvasRef = useRef<HTMLCanvasElement | null>(null);
  const [selectedRun, setSelectedRun] = useState('');
  const [compareRun, setCompareRun] = useState('');
  const [selectedAgent, setSelectedAgent] = useState('');
  const [events, setEvents] = useState<WorldEventItem[]>([]);
  const [fromSeq, setFromSeq] = useState(0);
  const [compareEvents, setCompareEvents] = useState<WorldEventItem[]>([]);
  const [compareFromSeq, setCompareFromSeq] = useState(0);
  const [timelineFilter, setTimelineFilter] = useState<'all' | 'incidents' | 'movement' | 'system'>('incidents');
  const [replayMode, setReplayMode] = useState(false);
  const [isPlaying, setIsPlaying] = useState(false);
  const [speed, setSpeed] = useState(2);
  const [scrubStep, setScrubStep] = useState(0);
  const [toggles, setToggles] = useState<OverlayToggles>({
    policy: true,
    rate: true,
    collision: true,
    loop: true,
    intention: true,
  });

  const runsQ = useQuery({ queryKey: ['world-runs'], queryFn: () => getWorldRuns(150), refetchInterval: 5000 });
  const worldEventsQ = useQuery({
    queryKey: ['world-events', selectedRun, fromSeq],
    queryFn: () => getWorldEvents(selectedRun, fromSeq, 200),
    enabled: selectedRun !== '',
    refetchInterval: replayMode ? false : 1200,
  });
  const compareWorldEventsQ = useQuery({
    queryKey: ['world-events-compare', compareRun, compareFromSeq],
    queryFn: () => getWorldEvents(compareRun, compareFromSeq, 200),
    enabled: compareRun !== '',
    refetchInterval: 2000,
  });
  const auditQ = useQuery({
    queryKey: ['world-audit', selectedRun],
    queryFn: () => getAuditEvents('world.move', 200),
    enabled: selectedRun !== '',
    refetchInterval: 2000,
  });
  const compareAuditQ = useQuery({
    queryKey: ['world-audit-compare', compareRun],
    queryFn: () => getAuditEvents('world.move', 200),
    enabled: compareRun !== '',
    refetchInterval: 3500,
  });
  const stateQ = useQuery({
    queryKey: ['world-state', selectedRun, replayMode ? scrubStep : 'latest'],
    queryFn: () => (replayMode ? getWorldState(selectedRun, scrubStep) : getWorldState(selectedRun)),
    enabled: selectedRun !== '',
    refetchInterval: replayMode ? false : 1200,
  });
  const compareStateQ = useQuery({
    queryKey: ['world-state-compare', compareRun, replayMode ? scrubStep : 'latest'],
    queryFn: () => (replayMode ? getWorldState(compareRun, scrubStep) : getWorldState(compareRun)),
    enabled: compareRun !== '',
    refetchInterval: replayMode ? false : 1800,
  });

  const createRunMutation = useMutation({
    mutationFn: () => createWorldRun(Date.now(), 50),
    onSuccess: (data) => {
      setSelectedRun(data.run_id);
      setReplayMode(false);
      setIsPlaying(false);
      setScrubStep(0);
      setEvents([]);
      setFromSeq(0);
      void runsQ.refetch();
    },
  });
  const replayMutation = useMutation({
    mutationFn: () => replayWorldRun(selectedRun),
    onSuccess: () => {
      void stateQ.refetch();
      void worldEventsQ.refetch();
    },
  });

  useEffect(() => {
    const first = runsQ.data?.items?.[0]?.run_id || '';
    if (selectedRun === '' && first !== '') {
      setSelectedRun(first);
    }
  }, [runsQ.data, selectedRun]);

  useEffect(() => {
    const runs = runsQ.data?.items || [];
    if (runs.length === 0) {
      if (compareRun !== '') setCompareRun('');
      return;
    }
    const exists = compareRun !== '' && runs.some((r) => r.run_id === compareRun);
    if (exists && compareRun !== selectedRun) {
      return;
    }
    const alt = runs.find((r) => r.run_id !== selectedRun)?.run_id || '';
    if (alt !== compareRun) {
      setCompareRun(alt);
    }
  }, [runsQ.data, selectedRun, compareRun]);

  useEffect(() => {
    setEvents([]);
    setFromSeq(0);
    setReplayMode(false);
    setIsPlaying(false);
    setScrubStep(0);
  }, [selectedRun]);

  useEffect(() => {
    setCompareEvents([]);
    setCompareFromSeq(0);
  }, [compareRun]);

  useEffect(() => {
    const items = worldEventsQ.data?.items || [];
    if (items.length === 0) {
      return;
    }
    setEvents((prev) => {
      const bySeq = new Map<number, WorldEventItem>();
      for (const ev of prev) bySeq.set(ev.seq, ev);
      for (const ev of items) bySeq.set(ev.seq, ev);
      return Array.from(bySeq.values()).sort((a, b) => a.seq - b.seq).slice(-2500);
    });
    const next = worldEventsQ.data?.next_seq || fromSeq;
    if (next > fromSeq) {
      setFromSeq(next);
    }
  }, [worldEventsQ.data, fromSeq]);

  useEffect(() => {
    const items = compareWorldEventsQ.data?.items || [];
    if (items.length === 0) {
      return;
    }
    setCompareEvents((prev) => {
      const bySeq = new Map<number, WorldEventItem>();
      for (const ev of prev) bySeq.set(ev.seq, ev);
      for (const ev of items) bySeq.set(ev.seq, ev);
      return Array.from(bySeq.values()).sort((a, b) => a.seq - b.seq).slice(-2500);
    });
    const next = compareWorldEventsQ.data?.next_seq || compareFromSeq;
    if (next > compareFromSeq) {
      setCompareFromSeq(next);
    }
  }, [compareWorldEventsQ.data, compareFromSeq]);

  const maxStep = useMemo(() => {
    let max = 0;
    for (const ev of events) {
      if (ev.step_id > max) max = ev.step_id;
    }
    return max;
  }, [events]);

  const compareMaxStep = useMemo(() => {
    let max = 0;
    for (const ev of compareEvents) {
      if (ev.step_id > max) max = ev.step_id;
    }
    return max;
  }, [compareEvents]);

  useEffect(() => {
    if (!replayMode || !isPlaying) return;
    const handle = window.setInterval(() => {
      setScrubStep((prev) => {
        if (prev >= maxStep) {
          setIsPlaying(false);
          return prev;
        }
        return Math.min(maxStep, prev + speed);
      });
    }, 450);
    return () => window.clearInterval(handle);
  }, [replayMode, isPlaying, maxStep, speed]);

  const worldState = stateQ.data?.state;
  const agents = useMemo(() => {
    const arr = [...((worldState?.agents || []) as WorldAgentState[])];
    arr.sort((a, b) => a.id.localeCompare(b.id));
    return arr;
  }, [worldState]);

  useEffect(() => {
    if (agents.length === 0) {
      setSelectedAgent('');
      return;
    }
    const exists = agents.some((a) => a.id === selectedAgent);
    if (!exists) {
      setSelectedAgent(agents[0].id);
    }
  }, [agents, selectedAgent]);

  const signals = useMemo(
    () => buildSignals(events, auditQ.data?.items || [], selectedRun),
    [events, auditQ.data, selectedRun],
  );
  const compareSignals = useMemo(
    () => (compareRun === '' ? emptySignals() : buildSignals(compareEvents, compareAuditQ.data?.items || [], compareRun)),
    [compareRun, compareEvents, compareAuditQ.data],
  );

  const selectedAgentState = agents.find((a) => a.id === selectedAgent) || null;
  const incident = useMemo(() => {
    return computeIncidentMetrics(events, signals, maxStep);
  }, [events, signals, maxStep]);
  const compareIncident = useMemo(() => {
    if (compareRun === '') return null;
    return computeIncidentMetrics(compareEvents, compareSignals, compareMaxStep);
  }, [compareRun, compareEvents, compareSignals, compareMaxStep]);
  const topImpactedAgents = useMemo(
    () => computeTopImpactedAgents(events, auditQ.data?.items || [], selectedRun),
    [events, auditQ.data, selectedRun],
  );
  const filteredWorldFeed = useMemo(() => {
    if (timelineFilter === 'all') return signals.worldFeed;
    if (timelineFilter === 'movement') {
      return signals.worldFeed.filter((e) => e.label === 'agent.moved');
    }
    if (timelineFilter === 'incidents') {
      return signals.worldFeed.filter((e) => e.label === 'agent.collided' || e.label === 'agent.blocked');
    }
    return signals.worldFeed.filter((e) => e.label === 'world.snapshot' || e.label === 'world.replayed' || e.label === 'world.seeded');
  }, [timelineFilter, signals.worldFeed]);
  const eventMix = useMemo(() => {
    const total = Math.max(1, incident.moved + incident.collisions + incident.denied + incident.rateLimited + incident.loops);
    return [
      { id: 'moved', label: 'Moved', value: incident.moved, pct: Math.round((incident.moved / total) * 100), tone: 'moved' },
      { id: 'collision', label: 'Collision', value: incident.collisions, pct: Math.round((incident.collisions / total) * 100), tone: 'collision' },
      { id: 'denied', label: 'Denied', value: incident.denied, pct: Math.round((incident.denied / total) * 100), tone: 'denied' },
      { id: 'rate', label: 'Rate', value: incident.rateLimited, pct: Math.round((incident.rateLimited / total) * 100), tone: 'rate' },
      { id: 'loop', label: 'Loop', value: incident.loops, pct: Math.round((incident.loops / total) * 100), tone: 'snapshot' },
    ];
  }, [incident]);

  useEffect(() => {
    if (!canvasRef.current) return;
    drawWorld(canvasRef.current, worldState, selectedAgent, signals, toggles);
  }, [worldState, selectedAgent, signals, toggles]);

  useEffect(() => {
    if (!compareCanvasRef.current || compareRun === '') return;
    drawWorld(compareCanvasRef.current, compareStateQ.data?.state, selectedAgent, compareSignals, toggles);
  }, [compareRun, compareStateQ.data, selectedAgent, compareSignals, toggles]);

  return (
    <div className="acuario-page">
      <section className="panel acuario-summary">
        <h2>Door Jam Snapshot</h2>
        <div className="acuario-kpis">
          <div className="kpi">
            <p>Agents</p>
            <strong>{agents.length}</strong>
          </div>
          <div className="kpi">
            <p>Moves</p>
            <strong>{incident.moved}</strong>
          </div>
          <div className="kpi">
            <p>Collisions</p>
            <strong>{incident.collisions}</strong>
          </div>
          <div className="kpi">
            <p>Policy Denied</p>
            <strong>{incident.denied}</strong>
          </div>
          <div className="kpi">
            <p>Rate Limited</p>
            <strong>{incident.rateLimited}</strong>
          </div>
          <div className="kpi">
            <p>Loop Agents</p>
            <strong>{incident.loops}</strong>
          </div>
        </div>
        <div className="acuario-root-cause">
          <div>
            <p className="muted">Run Health</p>
            <p className={`severity severity-${incident.severity}`}>{incident.severity.toUpperCase()}</p>
          </div>
          <div>
            <p className="muted">Jam Index</p>
            <p><strong>{incident.jamIndex}</strong> collisions per move</p>
          </div>
          <div>
            <p className="muted">Throughput</p>
            <p><strong>{incident.throughput}</strong> moves/step</p>
          </div>
        </div>
        <div className="acuario-compare">
          <label>
            Compare with run
            <select value={compareRun} onChange={(e) => setCompareRun(e.target.value)}>
              <option value="">None</option>
              {(runsQ.data?.items || [])
                .filter((r) => r.run_id !== selectedRun)
                .map((run) => (
                  <option key={run.run_id} value={run.run_id}>
                    {run.run_id} · seed {run.seed}
                  </option>
                ))}
            </select>
          </label>
          {compareIncident && (
            <div className="compare-delta-grid">
              <p>Moves <strong>{incident.moved}</strong> <span className="delta-pos">{deltaLabel(incident.moved, compareIncident.moved)}</span></p>
              <p>Collisions <strong>{incident.collisions}</strong> <span className="delta-neg">{deltaLabel(incident.collisions, compareIncident.collisions)}</span></p>
              <p>Denied <strong>{incident.denied}</strong> <span className="delta-neg">{deltaLabel(incident.denied, compareIncident.denied)}</span></p>
              <p>Rate <strong>{incident.rateLimited}</strong> <span className="delta-neg">{deltaLabel(incident.rateLimited, compareIncident.rateLimited)}</span></p>
              <p>Loops <strong>{incident.loops}</strong> <span className="delta-neg">{deltaLabel(incident.loops, compareIncident.loops)}</span></p>
              <p>Throughput <strong>{incident.throughput}</strong> <span className="delta-pos">{deltaLabel(incident.throughput, compareIncident.throughput)}</span></p>
            </div>
          )}
        </div>
      </section>

      <div className="acuario-grid">
      <Card title="Acuario 3D">
        <QueryError error={runsQ.error || worldEventsQ.error || stateQ.error || compareStateQ.error || auditQ.error || compareWorldEventsQ.error || compareAuditQ.error} onRetry={() => {
          void runsQ.refetch();
          void worldEventsQ.refetch();
          void stateQ.refetch();
          void compareStateQ.refetch();
          void compareWorldEventsQ.refetch();
          void compareAuditQ.refetch();
          void auditQ.refetch();
        }} />

        <div className="acuario-toolbar">
          <label>
            Run
            <select value={selectedRun} onChange={(e) => setSelectedRun(e.target.value)}>
              {(runsQ.data?.items || []).map((run) => (
                <option key={run.run_id} value={run.run_id}>
                  {run.run_id} · seed {run.seed}
                </option>
              ))}
            </select>
          </label>

          <label>
            Agent
            <select value={selectedAgent} onChange={(e) => setSelectedAgent(e.target.value)}>
              {agents.map((ag) => (
                <option key={ag.id} value={ag.id}>
                  {ag.id}
                </option>
              ))}
            </select>
          </label>

          <button onClick={() => createRunMutation.mutate()} disabled={createRunMutation.isPending}>
            {createRunMutation.isPending ? 'Creating...' : 'Create Run'}
          </button>
          <button className="ghost" onClick={() => replayMutation.mutate()} disabled={selectedRun === '' || replayMutation.isPending}>
            {replayMutation.isPending ? 'Replaying...' : 'Replay Run'}
          </button>
        </div>

        <div className={`acuario-arena ${compareRun !== '' ? 'with-compare' : ''}`}>
          <div className="acuario-canvas-wrap">
            <p className="arena-label">Primary run</p>
            <canvas ref={canvasRef} className="acuario-canvas" />
          </div>
          {compareRun !== '' && (
            <div className="acuario-canvas-wrap compare">
              <p className="arena-label">Compare run</p>
              <canvas ref={compareCanvasRef} className="acuario-canvas acuario-canvas-compare" />
              {compareIncident && (
                <p className="muted compare-caption">
                  severity {compareIncident.severity.toUpperCase()} · jam {compareIncident.jamIndex} · throughput {compareIncident.throughput}
                </p>
              )}
            </div>
          )}
        </div>

        <div className="overlay-row">
          <label><input type="checkbox" checked={toggles.policy} onChange={() => setToggles((t) => ({ ...t, policy: !t.policy }))} /> Policy</label>
          <label><input type="checkbox" checked={toggles.rate} onChange={() => setToggles((t) => ({ ...t, rate: !t.rate }))} /> Rate</label>
          <label><input type="checkbox" checked={toggles.collision} onChange={() => setToggles((t) => ({ ...t, collision: !t.collision }))} /> Collision</label>
          <label><input type="checkbox" checked={toggles.loop} onChange={() => setToggles((t) => ({ ...t, loop: !t.loop }))} /> Loop</label>
          <label><input type="checkbox" checked={toggles.intention} onChange={() => setToggles((t) => ({ ...t, intention: !t.intention }))} /> Intention</label>
        </div>
        <p className="muted acuario-hint">
          Movement: moved {incident.moved} · collided {incident.collided} · blocked {incident.blocked}
        </p>
        <p className="muted acuario-hint">
          Visual key: collision = orange ring (thick/bright when recent), loop = magenta, policy = red, rate = yellow.
        </p>
      </Card>

      <Card title="Replay + Agent POV">
        <div className="replay-controls">
          <label>
            <input type="checkbox" checked={replayMode} onChange={(e) => {
              setReplayMode(e.target.checked);
              setIsPlaying(false);
            }} />
            Replay mode
          </label>
          <button className="ghost" onClick={() => setIsPlaying((p) => !p)} disabled={!replayMode}>
            {isPlaying ? 'Pause' : 'Play'}
          </button>
          <label>
            Speed
            <select value={speed} onChange={(e) => setSpeed(Number(e.target.value))}>
              <option value={1}>1x</option>
              <option value={2}>2x</option>
              <option value={4}>4x</option>
              <option value={8}>8x</option>
            </select>
          </label>
        </div>
        <input
          type="range"
          min={0}
          max={Math.max(0, maxStep)}
          value={Math.min(scrubStep, maxStep)}
          onChange={(e) => setScrubStep(Number(e.target.value))}
          disabled={!replayMode}
        />
        <p className="muted">
          Step {replayMode ? scrubStep : stateQ.data?.step_id || 0} / {maxStep} · state hash {stateQ.data?.state_hash || '-'}
        </p>

        {selectedAgentState ? (
          <div className="agent-panel">
            <p><strong>{selectedAgentState.id}</strong></p>
            <p>Position: ({selectedAgentState.x.toFixed(2)}, {selectedAgentState.y.toFixed(2)})</p>
            <p>Velocity: ({selectedAgentState.vx.toFixed(2)}, {selectedAgentState.vy.toFixed(2)})</p>
            <p>Heading: {selectedAgentState.heading.toFixed(3)}</p>
            <p>Intention: ({Number(selectedAgentState.intention_x || 0).toFixed(2)}, {Number(selectedAgentState.intention_y || 0).toFixed(2)})</p>
            {signals.policy.get(selectedAgent) && <p className="overlay-policy">Policy denied: {signals.policy.get(selectedAgent)}</p>}
            {signals.rate.get(selectedAgent) && <p className="overlay-rate">Rate limited: {signals.rate.get(selectedAgent)}</p>}
            {signals.collision.has(selectedAgent) && <p className="overlay-collision">Collision/blocked detected</p>}
            {signals.loops.has(selectedAgent) && <p className="overlay-loop">Loop pattern detected</p>}
          </div>
        ) : (
          <p className="muted">No selected agent.</p>
        )}
      </Card>

      <Card title="Congestion Heatmap">
        <div className="heatmap-list">
          {signals.heatmap.length === 0 && <p className="muted">No congestion data yet.</p>}
          {signals.heatmap.map((cell) => (
            <div key={cell.cell}>
              <strong>{cell.cell}</strong>
              <span>{cell.hits}</span>
            </div>
          ))}
        </div>
      </Card>

      <Card title="Event Mix">
        <div className="mix-list">
          {eventMix.map((item) => (
            <div key={item.id} className="mix-item">
              <p><span className={`event-pill event-pill-${item.tone}`}>{item.label}</span> <strong>{item.value}</strong></p>
              <div className="mix-bar">
                <span className={`mix-fill mix-fill-${item.tone}`} style={{ width: `${Math.max(4, item.pct)}%` }} />
              </div>
            </div>
          ))}
        </div>
      </Card>

      <Card title="Top Impacted Agents">
        <div className="impacted-list">
          {topImpactedAgents.length === 0 && <p className="muted">No impacted agents yet.</p>}
          {topImpactedAgents.map((row) => (
            <div key={row.agentID}>
              <p><strong>{row.agentID}</strong> <span>score {row.score}</span></p>
              <small>collisions {row.collisions} · denied {row.denied} · rate {row.rateLimited} · moved {row.moved}</small>
            </div>
          ))}
        </div>
      </Card>

      <Card title="World Timeline">
        <div className="timeline-filters">
          <button className={timelineFilter === 'incidents' ? '' : 'ghost'} onClick={() => setTimelineFilter('incidents')}>Incidents</button>
          <button className={timelineFilter === 'movement' ? '' : 'ghost'} onClick={() => setTimelineFilter('movement')}>Movement</button>
          <button className={timelineFilter === 'system' ? '' : 'ghost'} onClick={() => setTimelineFilter('system')}>System</button>
          <button className={timelineFilter === 'all' ? '' : 'ghost'} onClick={() => setTimelineFilter('all')}>All</button>
        </div>
        <ul className="timeline timeline-tight">
          {filteredWorldFeed.slice(0, 80).map((ev) => (
            <li key={ev.key}>
              <p>
                <strong className={`event-pill event-pill-${eventTone(ev.label)}`}>{ev.label}</strong> <span>{ev.at}</span>
              </p>
              <pre>{ev.detail}</pre>
            </li>
          ))}
        </ul>
      </Card>

      <Card title="Enforcement Feed">
        <ul className="timeline timeline-tight">
          {signals.enforcementFeed.length === 0 && <li><pre>No tool.denied/tool.rate_limited events for this run.</pre></li>}
          {signals.enforcementFeed.slice(0, 80).map((ev) => (
            <li key={ev.key}>
              <p>
                <strong className={`event-pill event-pill-${eventTone(ev.label)}`}>{ev.label}</strong> <span>{ev.at}</span>
              </p>
              <pre>{ev.detail}</pre>
            </li>
          ))}
        </ul>
      </Card>
      </div>
    </div>
  );
}
