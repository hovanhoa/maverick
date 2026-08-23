import * as React from 'react';
import clsx from 'clsx';

/** Rounds up to a "nice" number (1/2/5 * 10^n) so axis ticks land on clean values. */
function niceCeil(value: number): number {
  if (value <= 0) return 1;
  const exp = Math.floor(Math.log10(value));
  const base = value / 10 ** exp;
  const niceBase = base <= 1 ? 1 : base <= 2 ? 2 : base <= 5 ? 5 : 10;
  return niceBase * 10 ** exp;
}

export interface LineChartPoint {
  /** X-axis label, e.g. an ISO date. */
  x: string;
  /** Primary series value, plotted as the line/area. */
  y: number;
  /** Extra rows shown in the hover tooltip alongside the primary value. */
  meta?: { label: string; value: string }[];
}

const VW = 600;
const VH = 220;
const MARGIN = { top: 12, right: 12, bottom: 24, left: 48 };
const PLOT_W = VW - MARGIN.left - MARGIN.right;
const PLOT_H = VH - MARGIN.top - MARGIN.bottom;
const TICKS = 4;

/**
 * A single-series line/area trend chart. Deliberately hand-rolled (no chart
 * library dependency, matching this project's zero-dependency web console) -
 * plain SVG with a hover crosshair, following standard chart conventions:
 * thin 2px line, a light area wash, hairline gridlines, a direct end-label,
 * and a tooltip that supplements (never gates) the data already visible.
 */
export function LineChart({
  points,
  formatY,
  formatX = (x) => x,
  emptyMessage = 'No data for this period.'
}: {
  points: LineChartPoint[];
  formatY: (v: number) => string;
  formatX?: (x: string) => string;
  emptyMessage?: string;
}) {
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [hover, setHover] = React.useState<number | null>(null);

  if (points.length === 0) {
    return <p className="px-5 py-10 text-center text-sm text-neutral-400">{emptyMessage}</p>;
  }

  const maxY = niceCeil(Math.max(...points.map((p) => p.y)));
  const xAt = (i: number) => (points.length === 1 ? MARGIN.left + PLOT_W / 2 : MARGIN.left + (i / (points.length - 1)) * PLOT_W);
  const yAt = (v: number) => MARGIN.top + PLOT_H - (v / maxY) * PLOT_H;

  const linePath = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${xAt(i)} ${yAt(p.y)}`).join(' ');
  const areaPath = `${linePath} L ${xAt(points.length - 1)} ${MARGIN.top + PLOT_H} L ${xAt(0)} ${MARGIN.top + PLOT_H} Z`;

  const yTicks = Array.from({ length: TICKS + 1 }, (_, i) => (maxY / TICKS) * i);

  // Show at most ~6 x-axis labels, always including the first and last point.
  const labelStep = Math.max(1, Math.ceil(points.length / 6));
  const xLabelIndices = points.map((_, i) => i).filter((i) => i === 0 || i === points.length - 1 || i % labelStep === 0);

  const handleMove = (e: React.PointerEvent<SVGRectElement>) => {
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return;
    const ratio = (e.clientX - rect.left) / rect.width;
    const i = Math.round(ratio * (points.length - 1));
    setHover(Math.min(points.length - 1, Math.max(0, i)));
  };

  const active = hover !== null ? points[hover] : null;
  const last = points[points.length - 1];

  return (
    <div ref={containerRef} className="relative px-5 py-4">
      <svg viewBox={`0 0 ${VW} ${VH}`} className="w-full" style={{ height: 'auto' }} role="img" aria-label="Daily usage trend">
        {yTicks.map((t) => (
          <g key={t}>
            <line
              x1={MARGIN.left}
              x2={VW - MARGIN.right}
              y1={yAt(t)}
              y2={yAt(t)}
              className="stroke-neutral-200 dark:stroke-white/10"
              strokeWidth={1}
            />
            <text x={MARGIN.left - 8} y={yAt(t)} dy="0.32em" textAnchor="end" className="fill-neutral-400 text-[9px]">
              {formatY(t)}
            </text>
          </g>
        ))}

        {xLabelIndices.map((i) => (
          <text
            key={i}
            x={xAt(i)}
            y={VH - 6}
            textAnchor={i === 0 ? 'start' : i === points.length - 1 ? 'end' : 'middle'}
            className="fill-neutral-400 text-[9px]"
          >
            {formatX(points[i].x)}
          </text>
        ))}

        <path d={areaPath} className="fill-primary-600/10 dark:fill-primary-400/10" stroke="none" />
        <path d={linePath} className="stroke-primary-600 dark:stroke-primary-400" strokeWidth={2} fill="none" strokeLinejoin="round" strokeLinecap="round" />

        {/* Direct end-label: the line's own value needs no legend for a single series. */}
        <circle cx={xAt(points.length - 1)} cy={yAt(last.y)} r={4} className="fill-primary-600 dark:fill-primary-400 stroke-white dark:stroke-neutral-900" strokeWidth={2} />
        <text x={VW - MARGIN.right} y={yAt(last.y) - 10} textAnchor="end" className="fill-neutral-900 text-[10px] font-semibold dark:fill-white">
          {formatY(last.y)}
        </text>

        {active && (
          <g>
            <line x1={xAt(hover!)} x2={xAt(hover!)} y1={MARGIN.top} y2={MARGIN.top + PLOT_H} className="stroke-neutral-300 dark:stroke-white/20" strokeWidth={1} />
            <circle cx={xAt(hover!)} cy={yAt(active.y)} r={4} className="fill-primary-600 dark:fill-primary-400 stroke-white dark:stroke-neutral-900" strokeWidth={2} />
          </g>
        )}

        {/* Hit area spans the full plot - the crosshair should find the nearest X, not require landing on the line. */}
        <rect
          x={MARGIN.left}
          y={MARGIN.top}
          width={PLOT_W}
          height={PLOT_H}
          fill="transparent"
          onPointerMove={handleMove}
          onPointerLeave={() => setHover(null)}
        />
      </svg>

      {active && (
        <div
          className="pointer-events-none absolute top-2 rounded-md bg-neutral-900 px-2.5 py-1.5 text-xs text-white shadow-lg dark:bg-white dark:text-neutral-900"
          style={{
            left: `${(xAt(hover!) / VW) * 100}%`,
            transform: hover! > points.length / 2 ? 'translateX(-100%)' : undefined
          }}
        >
          <p className="font-semibold">{formatX(active.x)}</p>
          <p>{formatY(active.y)}</p>
          {active.meta?.map((m) => (
            <p key={m.label} className="text-neutral-300 dark:text-neutral-600">
              {m.label}: {m.value}
            </p>
          ))}
        </div>
      )}
    </div>
  );
}

export interface BarListItem {
  key: string;
  label: string;
  value: number;
  meta?: string;
}

/**
 * A ranked magnitude comparison - "usage by X" breakdowns are all "compare
 * cost across categories", so one hue is enough (see dataviz skill: color
 * carries identity only when series ARE the subject; here the label text
 * already carries identity). Label, meta, and value are always visible
 * (never gated behind hover) - the title attribute is a native tooltip nicety.
 */
export function BarList({ items, formatValue }: { items: BarListItem[]; formatValue: (v: number) => string }) {
  if (items.length === 0) {
    return <p className="px-5 py-6 text-center text-sm text-neutral-400">No usage recorded yet this period.</p>;
  }

  const max = Math.max(...items.map((i) => i.value), 0.0001);

  return (
    <ul className="divide-y divide-neutral-950/5 px-5 py-2 dark:divide-white/10">
      {items.map((item) => (
        <li key={item.key} className="py-2.5" title={item.meta}>
          <div className="flex items-baseline justify-between gap-4">
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-neutral-900 dark:text-white">{item.label}</p>
              {item.meta && <p className="truncate text-xs text-neutral-500 dark:text-neutral-400">{item.meta}</p>}
            </div>
            <p className="shrink-0 text-sm font-semibold text-neutral-900 dark:text-white">{formatValue(item.value)}</p>
          </div>
          <div className="mt-1.5 h-2 overflow-hidden rounded-full bg-neutral-100 dark:bg-white/10">
            <div
              className={clsx('h-full rounded-full bg-primary-600 dark:bg-primary-500')}
              style={{ width: `${Math.max(2, Math.round((item.value / max) * 100))}%` }}
            />
          </div>
        </li>
      ))}
    </ul>
  );
}
